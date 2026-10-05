package aggregator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	ethmanTypes "github.com/0xPolygon/cdk/aggregator/ethmantypes"
	"github.com/0xPolygon/cdk/aggregator/prover"
	"github.com/0xPolygon/cdk/aggregator/validatorsign"
	rpctypes "github.com/0xPolygon/cdk/rpc/types"
	ethtxtypes "github.com/0xPolygon/zkevm-ethtx-manager/types"
	"github.com/0xPolygonHermez/zkevm-synchronizer-l1/state/entities"
	"github.com/ethereum/go-ethereum/common"
)

// configureValidatorSettlement installs the HTTP confirmer used when Validators is set.
// It runs only when SettlementBackend is "l1" and Validators is non-empty.
// That path does not start the ZK prover.
func (a *Aggregator) configureValidatorSettlement() {
	if !a.cfg.ValidatorSettlementEnabled() {
		return
	}
	a.validatorConfirmer = validatorsign.NewClient(nil)
	a.logger.Info("validator settlement enabled; ZK prover will not be used")
}

// settleSyncedBatchesWithValidators repeats settlement on a fixed interval.
// A failed BatchL2Data check, a validator rejection, or a tx send error is logged,
// then the same batch range is retried after the wait. There is no retry limit.
// The wait is VerifyProofInterval, or RetryTime when that interval is unset or zero.
func (a *Aggregator) settleSyncedBatchesWithValidators() {
	interval := a.cfg.VerifyProofInterval.Duration
	if interval <= 0 {
		interval = a.cfg.RetryTime.Duration
	}

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		// One round: load synced batches, check each BatchL2Data, collect validator
		// signatures, and submit the existing L1 function.
		if err := a.settleNextSyncedRange(a.ctx); err != nil {
			a.logger.Errorf("failed to settle L1-synced batches with validator signatures: %v", err)
		}

		select {
		case <-a.ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// settleNextSyncedRange settles the current virtualized batch range.
// A nil error and no transaction means there is nothing new, or the previous
// settlement is still waiting to show up on L1.
func (a *Aggregator) settleNextSyncedRange(ctx context.Context) error {
	// 1. Read the verified batch through the latest virtual batch and check BatchL2Data on each.
	req, ok, err := a.syncedBatchSettlement(ctx)
	if err != nil || !ok {
		return err
	}
	if a.validatorConfirmer == nil {
		return fmt.Errorf("validator confirmer is not configured")
	}

	// 2. POST each validator URL in order. The returned signer must equal that
	//    validator's ValidatorAddress. Any failure fails the round; the loop retries
	//    after the interval. The signature covers rollupId, the batch range,
	//    localExitRoot, stateRoot, and accInputHash.
	validators, err := a.cfg.ValidatorEndpoints()
	if err != nil {
		return err
	}
	confirmations, err := a.validatorConfirmer.Confirm(ctx, validators, *req)
	if err != nil {
		return err
	}

	// 3. Pack signatures into the existing verifyBatchesTrustedAggregator proof
	//    argument (bytes32[24]). At least (len(validators)+1)/2 signatures are
	//    required. Word 0 is the signature count, then each signature uses four
	//    words (signer, r, s, v).
	proof, err := validatorsign.EncodeTrustedProof(confirmations, len(validators))
	if err != nil {
		return err
	}

	// 4. Build the existing contract call. beneficiary is SenderAddress.
	//    initNumBatch is the current verified batch. finalNewBatch is the last
	//    virtual batch in this round. stateRoot and localExitRoot come from that
	//    last batch. accInputHash is only inside the signed digest, not a contract argument.
	beneficiary := common.HexToAddress(a.cfg.SenderAddress)
	inputs := &ethmanTypes.FinalProofInputs{
		FinalProof:       &prover.FinalProof{Proof: proof},
		NewLocalExitRoot: req.NewLocalExitRoot.Bytes(),
		NewStateRoot:     req.NewStateRoot.Bytes(),
	}
	to, data, err := a.etherman.BuildTrustedVerifyBatchesTxData(
		req.InitNumBatch, req.FinalNewBatch, inputs, beneficiary,
	)
	if err != nil {
		return fmt.Errorf("build validator settlement tx: %w", err)
	}

	// 5. Hand the calldata to the tx manager. Record the final batch first so the
	//    same range is not submitted again before the contract state updates.
	if _, err := a.ethTxManager.Add(ctx, to, big.NewInt(0), data, a.cfg.GasOffset, nil); err != nil {
		return fmt.Errorf("add validator settlement tx: %w", err)
	}
	a.pendingValidatorBatch.Store(req.FinalNewBatch)

	// 6. Wait until the tx is mined or fails. On failure, clear the pending mark
	//    and drop the monitored tx so the next round can resubmit.
	a.ethTxManager.ProcessPendingMonitoredTxs(ctx, func(result ethtxtypes.MonitoredTxResult) {
		if result.Status == ethtxtypes.MonitoredTxStatusFailed {
			a.logger.Errorf(
				"validator settlement tx %s failed for batches %d-%d",
				result.ID.Hex(), req.InitNumBatch, req.FinalNewBatch,
			)
			a.pendingValidatorBatch.Store(0)
			if err := a.ethTxManager.Remove(ctx, result.ID); err != nil {
				a.logger.Errorf("failed to remove validator settlement tx %s: %v", result.ID.Hex(), err)
			}

			return
		}
		a.logger.Infof(
			"validator settlement tx %s status %s for batches %d-%d",
			result.ID.Hex(), result.Status, req.InitNumBatch, req.FinalNewBatch,
		)
	})

	return nil
}

// syncedBatchSettlement loads the batch range for this round.
// (nil, false, nil) means wait for the next round: nothing new, a previous tx is
// still pending, a batch is not synced yet, or the final state root is still empty.
// A non-nil error is a check or read failure. The outer loop retries after the interval.
func (a *Aggregator) syncedBatchSettlement(ctx context.Context) (*validatorsign.Request, bool, error) {
	// 1. Read the latest batch already verified on the contract.
	//    The new range starts at lastVerified+1.
	lastVerified, err := a.etherman.GetLatestVerifiedBatchNum()
	if err != nil {
		return nil, false, fmt.Errorf("get latest verified batch: %w", err)
	}

	// 2. If the previous settlement's final batch is still ahead of the contract,
	//    that tx has not been applied yet. Wait for the next round.
	if pending := a.pendingValidatorBatch.Load(); pending > lastVerified {
		a.logger.Debugf("validator settlement for batch %d is still pending on L1", pending)
		return nil, false, nil
	}
	// The contract has caught up, so the pending mark is no longer needed.
	a.pendingValidatorBatch.Store(0)

	// 3. Read the newest virtual batch known to the synchronizer.
	//    One settlement covers lastVerified+1 through this batch.
	latestVirtual, err := a.l1Syncr.GetLastestVirtualBatchNumber(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("get latest virtual batch: %w", err)
	}
	// Nothing new has been virtualized since the last verified batch.
	if latestVirtual <= lastVerified {
		return nil, false, nil
	}

	// 4. Check BatchL2Data on every batch in the range, matching getAndLockBatchToProve.
	//    Any mismatch or unsynced batch blocks the whole range. Keep the last batch;
	//    the contract call carries only that batch's roots.
	var finalBatch *rpctypes.RPCBatch
	for batchNumber := lastVerified + 1; batchNumber <= latestVirtual; batchNumber++ {
		rpcBatch, err := a.checkBatchL2Data(ctx, batchNumber)
		if err != nil || rpcBatch == nil {
			return nil, false, err
		}
		finalBatch = rpcBatch
	}
	// 5. An empty state root means the L2 node has not finished executing the final batch.
	if finalBatch.StateRoot() == (common.Hash{}) {
		a.logger.Infof("synced batch %d state root is not ready", latestVirtual)
		return nil, false, nil
	}

	// 6. Read the accInputHash stored on L1 when the final batch was sequenced.
	//    Validators sign it as the identity of that batch. It is not a contract argument.
	accInputHash, err := a.etherman.GetBatchAccInputHash(ctx, latestVirtual)
	if err != nil {
		return nil, false, fmt.Errorf("get acc input hash for batch %d: %w", latestVirtual, err)
	}

	// 7. Build the payload validators sign: rollup id, the batch range, and the
	//    final batch's local exit root, state root, and accInputHash.
	return &validatorsign.Request{
		RollupID:         a.etherman.GetRollupId(),
		InitNumBatch:     lastVerified,
		FinalNewBatch:    latestVirtual,
		NewLocalExitRoot: finalBatch.LocalExitRoot(),
		NewStateRoot:     finalBatch.StateRoot(),
		AccInputHash:     accInputHash,
	}, true, nil
}

// checkBatchL2Data checks one batch.
// It compares the synchronizer's virtualBatch.BatchL2Data with the L2 RPC L2Data.
// Batch 1 is the injected batch and is skipped, matching getAndLockBatchToProve.
// (nil, nil) means the batch is not synced yet and the caller should wait for the next round.
func (a *Aggregator) checkBatchL2Data(ctx context.Context, batchNumber uint64) (*rpctypes.RPCBatch, error) {
	virtualBatch, err := a.l1Syncr.GetVirtualBatchByBatchNumber(ctx, batchNumber)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			a.logger.Infof("virtual batch %d has not been synced yet", batchNumber)
			return nil, nil
		}

		return nil, fmt.Errorf("get virtual batch %d: %w", batchNumber, err)
	}
	if virtualBatch == nil {
		a.logger.Infof("virtual batch %d has not been synced yet", batchNumber)
		return nil, nil
	}

	rpcBatch, err := a.rpcClient.GetBatch(batchNumber)
	if err != nil {
		return nil, fmt.Errorf("get batch %d from RPC: %w", batchNumber, err)
	}
	// A byte mismatch fails the round. The outer loop retries after VerifyProofInterval
	// and does not ask validators or submit this range to L1.
	if batchNumber != 1 && !bytes.Equal(virtualBatch.BatchL2Data, rpcBatch.L2Data()) {
		a.logger.Warnf("BatchL2Data from virtual batch %d does not match the one from RPC", batchNumber)
		a.logger.Warnf("VirtualBatch BatchL2Data:%v", common.Bytes2Hex(virtualBatch.BatchL2Data))
		a.logger.Warnf("RPC BatchL2Data:%v", common.Bytes2Hex(rpcBatch.L2Data()))

		return nil, fmt.Errorf("batch %d BatchL2Data from L1 sync does not match RPC", batchNumber)
	}

	return rpcBatch, nil
}
