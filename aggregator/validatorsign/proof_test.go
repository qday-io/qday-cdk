package validatorsign

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestEncodeTrustedProofRequiresMajority(t *testing.T) {
	one := []Confirmation{{Signature: make([]byte, 65)}}

	encoded, err := EncodeTrustedProof(one, 1)
	require.NoError(t, err)
	raw, err := hexutil.Decode(encoded)
	require.NoError(t, err)
	require.Equal(t, byte(1), raw[31])

	_, err = EncodeTrustedProof(one, 3)
	require.Error(t, err)

	_, err = EncodeTrustedProof(append(one, Confirmation{Signature: make([]byte, 65)}), 3)
	require.NoError(t, err)

	tooMany := make([]Confirmation, maxValidatorSignatures+1)
	for i := range tooMany {
		tooMany[i].Signature = make([]byte, 65)
	}
	_, err = EncodeTrustedProof(tooMany, 1)
	require.Error(t, err)
}

func TestEncodeTrustedProofFitsExistingArgument(t *testing.T) {
	signer := common.HexToAddress("0x1000000000000000000000000000000000000001")
	sig := make([]byte, 65)
	sig[0] = 0xab
	sig[32] = 0xcd
	sig[64] = 27

	encoded, err := EncodeTrustedProof([]Confirmation{{Signer: signer, Signature: sig}}, 1)
	require.NoError(t, err)
	require.Len(t, encoded, 2+24*32*2)

	raw, err := hexutil.Decode(encoded)
	require.NoError(t, err)
	require.Len(t, raw, 24*32)
	require.Equal(t, byte(1), raw[31])
	require.Equal(t, signer.Bytes(), raw[32+12:64])
	require.Equal(t, byte(0xab), raw[64])
	require.Equal(t, byte(0xcd), raw[96])
	require.Equal(t, byte(27), raw[159])
}
