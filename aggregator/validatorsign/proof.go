package validatorsign

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// TrustedProofWords is the bytes32[24] proof argument of verifyBatchesTrustedAggregator.
	TrustedProofWords = 24
	// wordsPerSignature is address + r + s + v.
	wordsPerSignature = 4
	// maxValidatorSignatures fits in bytes32[24] after the leading count word.
	maxValidatorSignatures = (TrustedProofWords - 1) / wordsPerSignature
)

// EncodeTrustedProof packs validator signatures into the existing
// verifyBatchesTrustedAggregator proof argument. The L1 function is unchanged.
// The count written into word 0 is how many confirmations were passed in.
// That count must be at least (validatorCount+1)/2.
//
// Layout of the 24 words:
//
//	word 0: signature count
//	then each signature uses 4 words: signer address, r, s, v
func EncodeTrustedProof(confirmations []Confirmation, validatorCount int) (string, error) {
	minimum := (validatorCount + 1) / 2
	if len(confirmations) < minimum {
		return "", fmt.Errorf("validator signatures %d are below the minimum %d", len(confirmations), minimum)
	}
	if len(confirmations) > maxValidatorSignatures {
		return "", fmt.Errorf("validator signatures %d do not fit in the %d-word proof", len(confirmations), TrustedProofWords)
	}

	var proof [TrustedProofWords][32]byte
	binary.BigEndian.PutUint64(proof[0][24:], uint64(len(confirmations)))

	for i, confirmation := range confirmations {
		if len(confirmation.Signature) != crypto.SignatureLength {
			return "", fmt.Errorf("signature %d length %d, want %d", i, len(confirmation.Signature), crypto.SignatureLength)
		}
		base := 1 + i*wordsPerSignature
		copy(proof[base][12:], confirmation.Signer.Bytes())
		copy(proof[base+1][:], confirmation.Signature[:32])
		copy(proof[base+2][:], confirmation.Signature[32:64])
		proof[base+3][31] = confirmation.Signature[64]
	}

	raw := make([]byte, 0, TrustedProofWords*32)
	for i := range proof {
		raw = append(raw, proof[i][:]...)
	}

	return hexutil.Encode(raw), nil
}
