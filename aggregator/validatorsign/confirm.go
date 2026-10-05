// Package validatorsign requests signature confirmations for an L1-synced batch.
//
// Each configured validator is an HTTP endpoint. The aggregator POSTs the batch
// the L1 synchronizer has already virtualized, and the validator returns an
// Ethereum signature over that payload. There is no ZK proof.
//
// The signed digest is keccak256(abi.encode(
//
//	uint32 rollupID,
//	uint64 initNumBatch,
//	uint64 finalNewBatch,
//	bytes32 newLocalExitRoot,
//	bytes32 newStateRoot,
//	bytes32 accInputHash,
//
// )) wrapped as an Ethereum signed message
// ("\x19Ethereum Signed Message:\n32" || digest). The signatures are then packed
// into the existing verifyBatchesTrustedAggregator proof argument (bytes32[24]).
package validatorsign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	confirmTimeout = 30 * time.Second
	maxResponse    = 1 << 20
)

// Request is the L1-synced batch a validator must sign.
type Request struct {
	RollupID         uint32      `json:"rollupId"`
	InitNumBatch     uint64      `json:"initNumBatch"`
	FinalNewBatch    uint64      `json:"finalNewBatch"`
	NewLocalExitRoot common.Hash `json:"newLocalExitRoot"`
	NewStateRoot     common.Hash `json:"newStateRoot"`
	AccInputHash     common.Hash `json:"accInputHash"`
}

// Confirmation is one validator signature over Request.
type Confirmation struct {
	Signer    common.Address
	Signature []byte
}

type confirmationResponse struct {
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}

// Endpoint is one configured validator. Address is the signer the response must use.
type Endpoint struct {
	URL     string
	Address common.Address
}

// Confirmer collects a signature from every validator.
type Confirmer interface {
	Confirm(ctx context.Context, validators []Endpoint, req Request) ([]Confirmation, error)
}

// Client confirms batches over HTTP.
type Client struct {
	httpClient *http.Client
}

// NewClient returns an HTTP confirmer. A nil http.Client uses http.DefaultClient.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{httpClient: httpClient}
}

// Confirm POSTs req to every validator. The recovered signer must equal both the
// response signer and the configured ValidatorAddress.
func (c *Client) Confirm(ctx context.Context, validators []Endpoint, req Request) ([]Confirmation, error) {
	if len(validators) == 0 {
		return nil, fmt.Errorf("no validators configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode validator request: %w", err)
	}

	confirmations := make([]Confirmation, 0, len(validators))
	for _, validator := range validators {
		confirmation, err := c.confirmOne(ctx, validator, body, req)
		if err != nil {
			return nil, err
		}
		confirmations = append(confirmations, confirmation)
	}

	return confirmations, nil
}

func (c *Client) confirmOne(ctx context.Context, validator Endpoint, body []byte, req Request) (Confirmation, error) {
	url := strings.TrimSpace(validator.URL)
	callCtx, cancel := context.WithTimeout(ctx, confirmTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Confirmation{}, fmt.Errorf("build validator request %s: %w", url, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Confirmation{}, fmt.Errorf("request validator %s: %w", url, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return Confirmation{}, fmt.Errorf("read validator %s response: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return Confirmation{}, fmt.Errorf("validator %s returned %d: %s", url, resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var decoded confirmationResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Confirmation{}, fmt.Errorf("decode validator %s response: %w", url, err)
	}

	confirmation, err := decodeConfirmation(decoded)
	if err != nil {
		return Confirmation{}, fmt.Errorf("validator %s: %w", url, err)
	}
	if confirmation.Signer != validator.Address {
		return Confirmation{}, fmt.Errorf(
			"validator %s signer %s does not match validatorAddress %s",
			url, confirmation.Signer.Hex(), validator.Address.Hex(),
		)
	}
	if err := Verify(req, confirmation.Signer, confirmation.Signature); err != nil {
		return Confirmation{}, fmt.Errorf("validator %s: %w", url, err)
	}

	return confirmation, nil
}

func decodeConfirmation(resp confirmationResponse) (Confirmation, error) {
	if !common.IsHexAddress(resp.Signer) {
		return Confirmation{}, fmt.Errorf("invalid signer %q", resp.Signer)
	}
	sigHex := strings.TrimSpace(resp.Signature)
	if !strings.HasPrefix(sigHex, "0x") && !strings.HasPrefix(sigHex, "0X") {
		sigHex = "0x" + sigHex
	}
	sig, err := hexutil.Decode(sigHex)
	if err != nil {
		return Confirmation{}, fmt.Errorf("decode signature: %w", err)
	}

	return Confirmation{
		Signer:    common.HexToAddress(resp.Signer),
		Signature: sig,
	}, nil
}

// Digest is the abi.encode hash validators sign, before the Ethereum message prefix.
func Digest(req Request) (common.Hash, error) {
	encoded, err := digestArgs.Pack(
		req.RollupID,
		req.InitNumBatch,
		req.FinalNewBatch,
		req.NewLocalExitRoot,
		req.NewStateRoot,
		req.AccInputHash,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("encode settlement digest: %w", err)
	}

	return crypto.Keccak256Hash(encoded), nil
}

// EthSignedHash is the 32-byte hash passed to secp256k1, including the Ethereum signed-message prefix.
func EthSignedHash(req Request) ([]byte, error) {
	digest, err := Digest(req)
	if err != nil {
		return nil, err
	}

	return accounts.TextHash(digest.Bytes()), nil
}

// Verify checks that signature recovers to signer for req.
// The recovery id may be 0/1 or 27/28.
func Verify(req Request, signer common.Address, signature []byte) error {
	if len(signature) != crypto.SignatureLength {
		return fmt.Errorf("signature length %d, want %d", len(signature), crypto.SignatureLength)
	}

	hash, err := EthSignedHash(req)
	if err != nil {
		return err
	}

	sig := make([]byte, crypto.SignatureLength)
	copy(sig, signature)
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if sig[64] > 1 {
		return fmt.Errorf("invalid signature recovery id %d", signature[64])
	}

	pub, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return fmt.Errorf("recover validator signature: %w", err)
	}
	recovered := crypto.PubkeyToAddress(*pub)
	if recovered != signer {
		return fmt.Errorf("signature signer %s does not match recovered %s", signer.Hex(), recovered.Hex())
	}

	return nil
}

var digestArgs = mustDigestArgs()

func mustDigestArgs() abi.Arguments {
	uint32Ty, err := abi.NewType("uint32", "", nil)
	if err != nil {
		panic(err)
	}
	uint64Ty, err := abi.NewType("uint64", "", nil)
	if err != nil {
		panic(err)
	}
	bytes32Ty, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		panic(err)
	}

	return abi.Arguments{
		{Type: uint32Ty},
		{Type: uint64Ty},
		{Type: uint64Ty},
		{Type: bytes32Ty},
		{Type: bytes32Ty},
		{Type: bytes32Ty},
	}
}
