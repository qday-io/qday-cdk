package aggregator

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	ethmanTypes "github.com/0xPolygon/cdk/aggregator/ethmantypes"
	"github.com/0xPolygon/cdk/aggregator/mocks"
	"github.com/0xPolygon/cdk/aggregator/validatorsign"
	"github.com/0xPolygon/cdk/log"
	rpctypes "github.com/0xPolygon/cdk/rpc/types"
	"github.com/0xPolygonHermez/zkevm-synchronizer-l1/synchronizer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSettleNextSyncedRangeSendsValidatorSignatures(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signer := crypto.PubkeyToAddress(key.PublicKey)

	stateRoot := common.HexToHash("0x22")
	localExitRoot := common.HexToHash("0x11")
	accInputHash := common.HexToHash("0x33")
	l2Data := []byte{0x01, 0x02, 0x03}
	to := common.HexToAddress("0x24B3c7704709ed1491473F30393FFc93cFB0FC34")
	sender := "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"

	var seen validatorsign.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		hash, err := validatorsign.EthSignedHash(seen)
		require.NoError(t, err)
		sig, err := crypto.Sign(hash, key)
		require.NoError(t, err)
		sig[64] += 27
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{
			"signer":    signer.Hex(),
			"signature": hexutil.Encode(sig),
		}))
	}))
	defer srv.Close()

	etherman := mocks.NewEthermanMock(t)
	ethTxManager := mocks.NewEthTxManagerClientMock(t)
	rpcMock := mocks.NewRPCInterfaceMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(7), nil).Once()
	for batchNumber := uint64(5); batchNumber <= 7; batchNumber++ {
		batchL2Data := append([]byte(nil), l2Data...)
		batchL2Data = append(batchL2Data, byte(batchNumber))
		l1Syncr.On("GetVirtualBatchByBatchNumber", mock.Anything, batchNumber).
			Return(&synchronizer.VirtualBatch{BatchNumber: batchNumber, BatchL2Data: batchL2Data}, nil).Once()
		root := common.HexToHash("0x01")
		exitRoot := common.HexToHash("0x02")
		if batchNumber == 7 {
			root = stateRoot
			exitRoot = localExitRoot
		}
		rpcMock.On("GetBatch", batchNumber).Return(
			rpctypes.NewRPCBatch(batchNumber, accInputHash, nil, append([]byte(nil), batchL2Data...), common.Hash{}, exitRoot, root, common.Address{}, true),
			nil,
		).Once()
	}
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(7)).Return(accInputHash, nil).Once()
	etherman.On("GetRollupId").Return(uint32(1)).Once()
	etherman.On(
		"BuildTrustedVerifyBatchesTxData",
		uint64(4),
		uint64(7),
		mock.MatchedBy(func(inputs *ethmanTypes.FinalProofInputs) bool {
			if inputs == nil || inputs.FinalProof == nil {
				return false
			}
			if len(inputs.FinalProof.Proof) != 2+24*32*2 {
				return false
			}
			raw, err := hexutil.Decode(inputs.FinalProof.Proof)
			if err != nil || len(raw) < 64 {
				return false
			}

			return raw[31] == 1 && common.BytesToAddress(raw[32:64]) == signer
		}),
		common.HexToAddress(sender),
	).Return(&to, []byte{0x1, 0x2}, nil).Once()
	ethTxManager.On("Add", mock.Anything, &to, big.NewInt(0), []byte{0x1, 0x2}, uint64(0), mock.Anything).
		Return(common.Hash{}, nil).Once()
	ethTxManager.On("ProcessPendingMonitoredTxs", mock.Anything, mock.Anything).Return().Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend: L1,
			SenderAddress:     sender,
			Validators: []Validator{{
				URL:              srv.URL,
				ValidatorAddress: signer.Hex(),
			}},
		},
		logger:             log.GetDefaultLogger(),
		etherman:           etherman,
		ethTxManager:       ethTxManager,
		l1Syncr:            l1Syncr,
		rpcClient:          rpcMock,
		validatorConfirmer: validatorsign.NewClient(srv.Client()),
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(7), a.pendingValidatorBatch.Load())
	require.Equal(t, uint32(1), seen.RollupID)
	require.Equal(t, uint64(4), seen.InitNumBatch)
	require.Equal(t, uint64(7), seen.FinalNewBatch)
	require.Equal(t, stateRoot, seen.NewStateRoot)
	require.Equal(t, accInputHash, seen.AccInputHash)
}

func TestSettleNextSyncedRangeRejectsBatchL2DataMismatch(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	rpcMock := mocks.NewRPCInterfaceMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(7), nil).Once()
	l1Syncr.On("GetVirtualBatchByBatchNumber", mock.Anything, uint64(5)).
		Return(&synchronizer.VirtualBatch{BatchNumber: 5, BatchL2Data: []byte{0x01}}, nil).Once()
	rpcMock.On("GetBatch", uint64(5)).Return(
		rpctypes.NewRPCBatch(5, common.Hash{}, nil, []byte{0x01}, common.Hash{}, common.HexToHash("0x11"), common.HexToHash("0x22"), common.Address{}, true),
		nil,
	).Once()
	l1Syncr.On("GetVirtualBatchByBatchNumber", mock.Anything, uint64(6)).
		Return(&synchronizer.VirtualBatch{BatchNumber: 6, BatchL2Data: []byte{0x01}}, nil).Once()
	rpcMock.On("GetBatch", uint64(6)).Return(
		rpctypes.NewRPCBatch(6, common.Hash{}, nil, []byte{0x02}, common.Hash{}, common.HexToHash("0x11"), common.HexToHash("0x22"), common.Address{}, true),
		nil,
	).Once()

	a := &Aggregator{
		cfg: Config{SettlementBackend: L1, Validators: []Validator{{
			URL:              "http://validator",
			ValidatorAddress: "0x0000000000000000000000000000000000000001",
		}}},
		logger:    log.GetDefaultLogger(),
		etherman:  etherman,
		l1Syncr:   l1Syncr,
		rpcClient: rpcMock,
	}

	err := a.settleNextSyncedRange(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "batch 6 BatchL2Data")
	require.Equal(t, uint64(0), a.pendingValidatorBatch.Load())
}

func TestSettleNextSyncedRangeWaitsWhenNothingNew(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)
	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(7), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(7), nil).Once()

	a := &Aggregator{
		cfg: Config{SettlementBackend: L1, Validators: []Validator{{
			URL:              "http://validator",
			ValidatorAddress: "0x0000000000000000000000000000000000000001",
		}}},
		logger:   log.GetDefaultLogger(),
		etherman: etherman,
		l1Syncr:  l1Syncr,
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
}

func TestSettleNextSyncedRangeSkipsWhilePending(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()

	a := &Aggregator{
		cfg: Config{SettlementBackend: L1, Validators: []Validator{{
			URL:              "http://validator",
			ValidatorAddress: "0x0000000000000000000000000000000000000001",
		}}},
		logger:   log.GetDefaultLogger(),
		etherman: etherman,
	}
	a.pendingValidatorBatch.Store(9)

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(9), a.pendingValidatorBatch.Load())
}
