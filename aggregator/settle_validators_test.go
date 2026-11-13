package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	ethmanTypes "github.com/0xPolygon/cdk/aggregator/ethmantypes"
	"github.com/0xPolygon/cdk/aggregator/mocks"
	"github.com/0xPolygon/cdk/aggregator/validatorsign"
	"github.com/0xPolygon/cdk/log"
	rpctypes "github.com/0xPolygon/cdk/rpc/types"
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
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(7)).Return(accInputHash, nil).Once()
	rpcMock.On("GetBatch", uint64(7)).Return(
		rpctypes.NewRPCBatch(7, accInputHash, nil, nil, common.Hash{}, localExitRoot, stateRoot, common.Address{}, true),
		nil,
	).Once()
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
			SettlementBackend:    L1,
			SenderAddress:        sender,
			BatchesPerSettlement: 10,
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

func TestSettleNextSyncedRangeCapsAtBatchesPerSettlement(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	rpcMock := mocks.NewRPCInterfaceMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)
	accInputHash := common.HexToHash("0x33")
	stateRoot := common.HexToHash("0x22")
	localExitRoot := common.HexToHash("0x11")

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(10), nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(6)).Return(accInputHash, nil).Once()
	rpcMock.On("GetBatch", uint64(6)).Return(
		rpctypes.NewRPCBatch(6, accInputHash, nil, nil, common.Hash{}, localExitRoot, stateRoot, common.Address{}, true),
		nil,
	).Once()
	etherman.On("GetRollupId").Return(uint32(1)).Once()
	etherman.On("BuildTrustedVerifyBatchesTxData", uint64(4), uint64(6), mock.Anything, mock.Anything).
		Return(new(common.Address), []byte{0x1}, nil).Once()
	ethTxManager := mocks.NewEthTxManagerClientMock(t)
	ethTxManager.On("Add", mock.Anything, mock.Anything, big.NewInt(0), []byte{0x1}, uint64(0), mock.Anything).
		Return(common.Hash{}, nil).Once()
	ethTxManager.On("ProcessPendingMonitoredTxs", mock.Anything, mock.Anything).Return().Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend:    L1,
			BatchesPerSettlement: 2,
			SenderAddress:        "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
			Validators: []Validator{{
				URL:              "http://127.0.0.1:1",
				ValidatorAddress: "0x0000000000000000000000000000000000000001",
			}},
		},
		logger:             log.GetDefaultLogger(),
		etherman:           etherman,
		ethTxManager:       ethTxManager,
		l1Syncr:            l1Syncr,
		rpcClient:          rpcMock,
		validatorConfirmer: acceptingConfirmer{},
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(6), a.pendingValidatorBatch.Load())
}

func TestSettleNextSyncedRangeSkipsEmptyAccInputHash(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	rpcMock := mocks.NewRPCInterfaceMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)
	accInputHash := common.HexToHash("0x33")
	stateRoot := common.HexToHash("0x22")
	localExitRoot := common.HexToHash("0x11")

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(7), nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(7)).Return(common.Hash{}, nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(6)).Return(accInputHash, nil).Once()
	rpcMock.On("GetBatch", uint64(6)).Return(
		rpctypes.NewRPCBatch(6, accInputHash, nil, nil, common.Hash{}, localExitRoot, stateRoot, common.Address{}, true),
		nil,
	).Once()
	etherman.On("GetRollupId").Return(uint32(1)).Once()
	etherman.On("BuildTrustedVerifyBatchesTxData", uint64(4), uint64(6), mock.Anything, mock.Anything).
		Return(new(common.Address), []byte{0x1}, nil).Once()
	ethTxManager := mocks.NewEthTxManagerClientMock(t)
	ethTxManager.On("Add", mock.Anything, mock.Anything, big.NewInt(0), []byte{0x1}, uint64(0), mock.Anything).
		Return(common.Hash{}, nil).Once()
	ethTxManager.On("ProcessPendingMonitoredTxs", mock.Anything, mock.Anything).Return().Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend:    L1,
			BatchesPerSettlement: 10,
			SenderAddress:        "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
			Validators: []Validator{{
				URL:              "http://127.0.0.1:1",
				ValidatorAddress: "0x0000000000000000000000000000000000000001",
			}},
		},
		logger:             log.GetDefaultLogger(),
		etherman:           etherman,
		ethTxManager:       ethTxManager,
		l1Syncr:            l1Syncr,
		rpcClient:          rpcMock,
		validatorConfirmer: acceptingConfirmer{},
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(6), a.pendingValidatorBatch.Load())
}

func TestSettleNextSyncedRangeWaitsWhenAccInputHashEmpty(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(6), nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(6)).Return(common.Hash{}, nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(5)).Return(common.Hash{}, nil).Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend:    L1,
			BatchesPerSettlement: 10,
			Validators: []Validator{{
				URL:              "http://validator",
				ValidatorAddress: "0x0000000000000000000000000000000000000001",
			}},
		},
		logger:   log.GetDefaultLogger(),
		etherman: etherman,
		l1Syncr:  l1Syncr,
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(0), a.pendingValidatorBatch.Load())
}

func TestSettleNextSyncedRangeExtendsLimitWhenAccInputHashEmpty(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	rpcMock := mocks.NewRPCInterfaceMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)
	accInputHash := common.HexToHash("0x33")
	stateRoot := common.HexToHash("0x22")
	localExitRoot := common.HexToHash("0x11")

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(10), nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(6)).Return(common.Hash{}, nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(5)).Return(common.Hash{}, nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(8)).Return(accInputHash, nil).Once()
	rpcMock.On("GetBatch", uint64(8)).Return(
		rpctypes.NewRPCBatch(8, accInputHash, nil, nil, common.Hash{}, localExitRoot, stateRoot, common.Address{}, true),
		nil,
	).Once()
	etherman.On("GetRollupId").Return(uint32(1)).Once()
	etherman.On("BuildTrustedVerifyBatchesTxData", uint64(4), uint64(8), mock.Anything, mock.Anything).
		Return(new(common.Address), []byte{0x1}, nil).Once()
	ethTxManager := mocks.NewEthTxManagerClientMock(t)
	ethTxManager.On("Add", mock.Anything, mock.Anything, big.NewInt(0), []byte{0x1}, uint64(0), mock.Anything).
		Return(common.Hash{}, nil).Once()
	ethTxManager.On("ProcessPendingMonitoredTxs", mock.Anything, mock.Anything).Return().Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend:    L1,
			BatchesPerSettlement: 2,
			SenderAddress:        "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
			Validators: []Validator{{
				URL:              "http://127.0.0.1:1",
				ValidatorAddress: "0x0000000000000000000000000000000000000001",
			}},
		},
		logger:             log.GetDefaultLogger(),
		etherman:           etherman,
		ethTxManager:       ethTxManager,
		l1Syncr:            l1Syncr,
		rpcClient:          rpcMock,
		validatorConfirmer: acceptingConfirmer{},
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(8), a.pendingValidatorBatch.Load())
}

func TestSettleNextSyncedRangeWaitsWhenStateRootEmpty(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	rpcMock := mocks.NewRPCInterfaceMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)
	accInputHash := common.HexToHash("0x33")

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(10), nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(6)).Return(accInputHash, nil).Once()
	rpcMock.On("GetBatch", uint64(6)).Return(
		rpctypes.NewRPCBatch(6, accInputHash, nil, nil, common.Hash{}, common.HexToHash("0x11"), common.Hash{}, common.Address{}, true),
		nil,
	).Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend:    L1,
			BatchesPerSettlement: 2,
			Validators: []Validator{{
				URL:              "http://validator",
				ValidatorAddress: "0x0000000000000000000000000000000000000001",
			}},
		},
		logger:    log.GetDefaultLogger(),
		etherman:  etherman,
		l1Syncr:   l1Syncr,
		rpcClient: rpcMock,
	}

	require.NoError(t, a.settleNextSyncedRange(context.Background()))
	require.Equal(t, uint64(0), a.pendingValidatorBatch.Load())
}

func TestSettleNextSyncedRangeRetriesWhenReadFails(t *testing.T) {
	etherman := mocks.NewEthermanMock(t)
	l1Syncr := mocks.NewSynchronizerInterfaceMock(t)

	etherman.On("GetLatestVerifiedBatchNum").Return(uint64(4), nil).Once()
	l1Syncr.On("GetLastestVirtualBatchNumber", mock.Anything).Return(uint64(10), nil).Once()
	etherman.On("GetBatchAccInputHash", mock.Anything, uint64(6)).Return(common.Hash{}, fmt.Errorf("rpc down")).Once()

	a := &Aggregator{
		cfg: Config{
			SettlementBackend:    L1,
			BatchesPerSettlement: 2,
			Validators: []Validator{{
				URL:              "http://validator",
				ValidatorAddress: "0x0000000000000000000000000000000000000001",
			}},
		},
		logger:   log.GetDefaultLogger(),
		etherman: etherman,
		l1Syncr:  l1Syncr,
	}

	err := a.settleNextSyncedRange(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "batch 6")
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

type acceptingConfirmer struct{}

func (acceptingConfirmer) Confirm(
	context.Context, []validatorsign.Endpoint, validatorsign.Request,
) ([]validatorsign.Confirmation, error) {
	return []validatorsign.Confirmation{{
		Signer:    common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Signature: make([]byte, crypto.SignatureLength),
	}}, nil
}
