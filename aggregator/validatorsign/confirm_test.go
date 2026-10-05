package validatorsign

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestConfirmRecoversSigner(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signer := crypto.PubkeyToAddress(key.PublicKey)

	req := Request{
		RollupID:         7,
		InitNumBatch:     4,
		FinalNewBatch:    9,
		NewLocalExitRoot: common.HexToHash("0x11"),
		NewStateRoot:     common.HexToHash("0x22"),
		AccInputHash:     common.HexToHash("0x33"),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var got Request
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		require.Equal(t, req, got)

		hash, err := EthSignedHash(got)
		require.NoError(t, err)
		sig, err := crypto.Sign(hash, key)
		require.NoError(t, err)
		sig[64] += 27

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(confirmationResponse{
			Signer:    signer.Hex(),
			Signature: hexutil.Encode(sig),
		}))
	}))
	defer srv.Close()

	got, err := NewClient(srv.Client()).Confirm(context.Background(), []Endpoint{{URL: srv.URL, Address: signer}}, req)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, signer, got[0].Signer)
	require.Len(t, got[0].Signature, crypto.SignatureLength)
}

func TestConfirmRejectsBadSignature(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	other, err := crypto.GenerateKey()
	require.NoError(t, err)

	req := Request{RollupID: 1, FinalNewBatch: 2, NewStateRoot: common.HexToHash("0x01")}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash, err := EthSignedHash(req)
		require.NoError(t, err)
		sig, err := crypto.Sign(hash, other)
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(w).Encode(confirmationResponse{
			Signer:    crypto.PubkeyToAddress(key.PublicKey).Hex(),
			Signature: hexutil.Encode(sig),
		}))
	}))
	defer srv.Close()

	_, err = NewClient(srv.Client()).Confirm(context.Background(), []Endpoint{{
		URL:     srv.URL,
		Address: crypto.PubkeyToAddress(key.PublicKey),
	}}, req)
	require.Error(t, err)
}

func TestConfirmRejectsSignerMismatch(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signer := crypto.PubkeyToAddress(key.PublicKey)

	req := Request{RollupID: 1, FinalNewBatch: 2, NewStateRoot: common.HexToHash("0x01")}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash, err := EthSignedHash(req)
		require.NoError(t, err)
		sig, err := crypto.Sign(hash, key)
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(w).Encode(confirmationResponse{
			Signer:    signer.Hex(),
			Signature: hexutil.Encode(sig),
		}))
	}))
	defer srv.Close()

	_, err = NewClient(srv.Client()).Confirm(context.Background(), []Endpoint{{
		URL:     srv.URL,
		Address: common.HexToAddress("0x0000000000000000000000000000000000000001"),
	}}, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validatorAddress")
}

func TestVerifyAcceptsRecoveryIDZero(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	req := Request{RollupID: 3, InitNumBatch: 1, FinalNewBatch: 2, AccInputHash: common.HexToHash("0xaa")}
	hash, err := EthSignedHash(req)
	require.NoError(t, err)
	sig, err := crypto.Sign(hash, key)
	require.NoError(t, err)

	require.NoError(t, Verify(req, crypto.PubkeyToAddress(key.PublicKey), sig))
}
