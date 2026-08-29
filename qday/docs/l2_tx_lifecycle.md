# L2 Transaction Lifecycle Troubleshooting

Given a transaction hash, determine which stage an L2 transaction is in and which component is blocking it.

L2 RPC defaults to cdk-erigon (`$L2_RPC` below). The L1 RPC used by the aggregator and sequence-sender is `$L1_RPC`. It must match `cdk-node-config.toml`. Do not use the L2 endpoint (for example `localhost:8545`) as L1.

## Lifecycle

```
Pending → Trusted → Virtual → Verified
   │         │          │          │
 mempool   L2 block    sequence-   aggregator
            + batch    sender      + prover
                       to L1       proof on L1
```

| Stage | Meaning | Advanced by | How to tell |
|------|---------|-------------|-------------|
| Pending | Still in the mempool, not in a block | Sequencer | `eth_getTransactionByHash` returns the tx, `blockNumber` is `null` |
| Trusted | Included in an L2 block / batch and executed locally | cdk-erigon | Receipt exists; batch `batchNumber` ≤ `zkevm_batchNumber` |
| Virtual | Batch submitted to L1 by sequence-sender | sequence-sender | `batchNumber` ≤ `zkevm_virtualBatchNumber`; `sendSequencesTxHash` is set |
| Verified | ZK proof verified on L1 (final) | aggregator + prover | `batchNumber` ≤ `zkevm_verifiedBatchNumber`; `verifyBatchTxHash` is set |

Resolve the tx to a `batchNumber`, then compare it to the three watermarks:

- `batchNumber` ≤ `zkevm_verifiedBatchNumber` → **Verified**
- else `batchNumber` ≤ `zkevm_virtualBatchNumber` → **Virtual** (waiting for proof)
- else a receipt exists → **Trusted** (waiting for sequence-sender)
- tx exists but `blockNumber` is empty → **Pending**
- tx not found on L2 → wrong hash, not in this node's txpool, or the node is not synced

## 1. Transaction hash → batch

```bash
TX=0x...
L2_RPC=http://localhost:8545

# Does the tx exist, and is it in a block?
cast tx "$TX" --rpc-url "$L2_RPC"
cast receipt "$TX" --rpc-url "$L2_RPC"
```

- Tx not found: invalid hash, or it has not reached this node's txpool.
- `blockNumber` is empty: still **Pending**. Check the L2 txpool, nonce, and gas. CDK (sequence-sender / aggregator) is not involved yet.
- `blockNumber` is set: continue.

```bash
BLOCK=$(cast receipt "$TX" --rpc-url "$L2_RPC" --json | jq -r '.blockNumber')
BATCH_HEX=$(cast rpc --rpc-url "$L2_RPC" zkevm_batchNumberByBlockNumber "$BLOCK" | tr -d '"')
BATCH=$(cast to-dec "$BATCH_HEX")
echo "block=$BLOCK batch=$BATCH"
```

## 2. Watermarks and batch details

```bash
cast rpc --rpc-url "$L2_RPC" zkevm_batchNumber
cast rpc --rpc-url "$L2_RPC" zkevm_virtualBatchNumber
cast rpc --rpc-url "$L2_RPC" zkevm_verifiedBatchNumber
cast rpc --rpc-url "$L2_RPC" zkevm_getBatchByNumber "$BATCH"
```

Important fields on `zkevm_getBatchByNumber`:

| Field | Meaning |
|------|---------|
| `closed` | Whether the batch is closed. Sequence-sender will not submit an open batch |
| `sendSequencesTxHash` | L1 tx from sequence-sender. `null` = not Virtual yet |
| `verifyBatchTxHash` | L1 verify tx from the aggregator. `null` = not Verified yet |
| `accInputHash` | Often all zeros before sequencing; should be non-zero after Virtual |
| `transactions` / `blocks` | Confirm the hash is actually in this batch |

One-shot stage script:

```bash
TX=0x...
L2_RPC=http://localhost:8545

BLOCK=$(cast receipt "$TX" --rpc-url "$L2_RPC" --json | jq -r '.blockNumber')
[ "$BLOCK" = "null" ] && echo "stage=Pending" && exit 0

BATCH_HEX=$(cast rpc --rpc-url "$L2_RPC" zkevm_batchNumberByBlockNumber "$BLOCK" | tr -d '"')
BATCH=$(cast to-dec "$BATCH_HEX")

TRUSTED=$(cast to-dec "$(cast rpc --rpc-url "$L2_RPC" zkevm_batchNumber | tr -d '"')")
VIRTUAL=$(cast to-dec "$(cast rpc --rpc-url "$L2_RPC" zkevm_virtualBatchNumber | tr -d '"')")
VERIFIED=$(cast to-dec "$(cast rpc --rpc-url "$L2_RPC" zkevm_verifiedBatchNumber | tr -d '"')")

echo "tx=$TX block=$BLOCK batch=$BATCH"
echo "trusted=$TRUSTED virtual=$VIRTUAL verified=$VERIFIED"

if   [ "$BATCH" -le "$VERIFIED" ]; then echo "stage=Verified"
elif [ "$BATCH" -le "$VIRTUAL"  ]; then echo "stage=Virtual (waiting proof)"
else echo "stage=Trusted (waiting sequence-sender)"
fi

cast rpc --rpc-url "$L2_RPC" zkevm_getBatchByNumber "$BATCH" | jq '{
  number, closed, accInputHash, sendSequencesTxHash, verifyBatchTxHash
}'
```

Backlog:

```
Trusted  − Virtual  = batches waiting for sequence-sender to submit to L1
Virtual  − Verified = batches waiting for the aggregator to prove
your batch − Virtual / Verified = how many batches this tx still has to wait
```

## 3. Backlog vs stuck

Sample the three watermarks again after 1–2 minutes.

| Observation | Conclusion |
|-------------|------------|
| Trusted is increasing | Sequencer is healthy |
| Virtual is increasing | Sequence-sender is working, possibly lagging |
| Verified is increasing | Aggregator is settling |
| Virtual increases, Verified does not | Proof pipeline (aggregator / prover / L1 finality) is the bottleneck |
| All three frozen | Check that L1 is producing blocks and RPCs are reachable |
| Watermarks pause, then jump by tens of batches | Typical when the aggregator syncs only L1 **finalized** (see below) |

`zkevm_verifiedBatchNumber` on L2 RPC can lag the L1 contract. The aggregator log line `Last Verified Batch Number` comes from `etherman.GetLatestVerifiedBatchNum()` and is the source of truth for L1 progress.

## 4. Debug by stuck stage

### Stuck in Pending

Check the L2 node's txpool, account nonce, gas, and sync status. Sequence-sender and aggregator are not involved.

### Stuck in Trusted (`sendSequencesTxHash` is null)

Sequence-sender has not submitted this batch to L1.

```bash
# Logs
docker logs -f cdk-sequence-sender
# or the actual compose service name

# Local L1 send queue
docker exec cdk-sequence-sender sqlite3 /app/data/ethtxmanager.db \
  "SELECT * FROM monitored_txs;"
```

Look for: `sending sequence`, `latest virtual batch is`, L1 RPC errors, POL allowance, `L1BlockTimestampMargin`.

Confirm `closed` is `true` on `zkevm_getBatchByNumber`. Open batches are not sent.

Common causes: L1 RPC unreachable, sequencer has not approved the rollup contract to spend POL, or `InitialBlock` is wrong so L1InfoTree sync fails. See [usage.md](usage.md).

### Stuck in Virtual (`verifyBatchTxHash` is null)

The batch is sequenced on L1. The aggregator still needs to prove it and call `verifyBatches`.

Query chain head on the **aggregator's L1 RPC** (`Aggregator.Synchronizer.Etherman.L1URL` / `Aggregator.EthTxManager.Etherman.URL`), not the L2 port:

```bash
L1_RPC=http://10.x.x.x:8545   # URL from config

cast block latest --rpc-url "$L1_RPC" --json | jq .number
cast block finalized --rpc-url "$L1_RPC" --json | jq .number
```

Default config:

```
[Aggregator.Synchronizer.Synchronizer]
SyncUpToBlock = "finalized"
BlockFinality = "finalized"
```

The aggregator only consumes **finalized** L1 blocks. On a PoS L1, `finalized` usually jumps by epoch (32 slots) and lags `latest` by tens of minutes. Verified progress then looks like: **pause → catch up tens of batches → pause again**. That is not necessarily a failure.

Aggregator log lines (line numbers refer to the current code):

| Log | Meaning |
|-----|---------|
| `Sequencing event for batch N has not been synced yet` | Local L1 sync DB has no `SequenceBatches` event for batch N, so proving cannot start |
| `lastBlockSynced` / `finalizedBlock` / `MaximumBlock` | L1 sync progress. Should track `$L1_RPC` `finalized` |
| `New block. BlockNumber: ...` | Only L1 blocks that contain rollup events are stored; not every L1 block logs this |
| `NetworkID 0 Synced!` | Caught up to the current target (usually finalized). Truly stuck only if both `lastBlockSynced` and `MaximumBlock` stay unchanged |
| `All information to generate proof for batch N is ready` | Event is synced; witness fetch starts |
| `Sending zki + batch to the prover, batchNumber [N]` | Handed to the prover |
| `Last Verified Batch Number:N` | Latest verified batch on the L1 contract |

**Healthy L1 sync vs stuck:**

- `MaximumBlock` ≈ on-chain `finalized`, `lastBlockSynced` close to it, a few blocks with no `New block`: at the tip, waiting for the next finality window or the next `SequenceBatches`. Normal.
- `MaximumBlock` lags on-chain `finalized` for a long time, or the same `FromBlock–ToBlock` range is scanned while `lastBlockSynced` never moves: syncer failed to process a block, RPC is stale, or a reorg. Search logs for `error`, `reorg`, `rollback`, `parent hash`.
- If it still does not advance, back up and delete the aggregator's L1 sync DB (`aggregator_sync_db.sqlite`). It will resync from `GenesisBlockNumber`.

**Prover:** a `proverAddr` in the logs means gRPC is connected. Restarting the prover while seeing `Waiting ... sequencing event` does not help; the L1 event has not reached the aggregator yet.

Confirm the two L1 transactions:

```bash
# sequenced
cast receipt <sendSequencesTxHash> --rpc-url "$L1_RPC"

# verified (once the field is set)
cast receipt <verifyBatchTxHash> --rpc-url "$L1_RPC"
```

Aggregator L1 send queue:

```bash
docker exec cdk-aggregator sqlite3 /app/data/aggregator_ethtx.db \
  "SELECT * FROM monitored_txs;"
```

### Already Verified on L1, but explorer / RPC still shows unconfirmed

The L2 node may lag while syncing L1 verify events. Trust the L1 contract and the `verifyBatchTxHash` receipt, and wait for cdk-erigon to catch up.

## 5. Example (testnet walkthrough)

The transaction was in batch `0x3bed` (15341).

Initially:

```
zkevm_getBatchByNumber 0x3bed
  closed=true, sendSequencesTxHash=null, verifyBatchTxHash=null, accInputHash=0x00..00

zkevm_batchNumber          0x3c1a  (15386)  Trusted ahead
zkevm_virtualBatchNumber   0x3b98  (15256)  85 batches behind
zkevm_verifiedBatchNumber  0x3af9  (15097)  further behind
```

Stage was **Trusted**. Virtual then moved from `0x3b98` to `0x3bb0` while Verified stayed flat → sequence-sender was catching up; the aggregator side looked stuck.

Aggregator logs were waiting on `Sequencing event for batch 15281` with `lastBlockSynced=28825` and `finalized=28832`. The cause was **sync-to-finalized**: L1 latest ≈ 29067, finalized `0x7100` (28928), about 140 blocks / 28 minutes behind, jumping 32 blocks per epoch.

After finality advanced, `New block` appeared (28807 / 28816 / 28825), then `Sending zki + batch to the prover, batchNumber [15281]`. The waiting batch moved 15281 → 15303 → 15380, so proof generation was progressing.

Later:

```
zkevm_virtualBatchNumber   0x3c2b  (15403)
zkevm_verifiedBatchNumber  0x3b60  (15200)   matches Last Verified Batch Number in logs

zkevm_getBatchByNumber 0x3bed
  sendSequencesTxHash=0xc7641e9d...   # now Virtual
  accInputHash=0x17c3dacb...          # non-zero
  verifyBatchTxHash=null              # still not Verified, 141 batches behind
```

The tx had reached **Virtual**. The aggregator was generating a proof for 15380 (already past 15341). Remaining work is submitting proofs to L1 until `zkevm_verifiedBatchNumber` ≥ 15341 and `verifyBatchTxHash` is set.

## 6. Command cheat sheet

```bash
# L2 watermarks
cast rpc --rpc-url "$L2_RPC" zkevm_batchNumber
cast rpc --rpc-url "$L2_RPC" zkevm_virtualBatchNumber
cast rpc --rpc-url "$L2_RPC" zkevm_verifiedBatchNumber

# tx → block → batch
cast receipt "$TX" --rpc-url "$L2_RPC"
cast rpc --rpc-url "$L2_RPC" zkevm_batchNumberByBlockNumber "$BLOCK"
cast rpc --rpc-url "$L2_RPC" zkevm_getBatchByNumber "$BATCH"

# L1 (must be the URL configured for aggregator / sequence-sender)
cast block latest --rpc-url "$L1_RPC" --json | jq .number
cast block finalized --rpc-url "$L1_RPC" --json | jq .number
cast receipt "$SEND_SEQ_TX" --rpc-url "$L1_RPC"
cast receipt "$VERIFY_TX" --rpc-url "$L1_RPC"

# Component logs
docker logs -f cdk-sequence-sender
docker logs -f cdk-aggregator
docker logs -f cdk-zkevm-prover
```

Aggregator log search terms: `Sequencing event`, `lastBlockSynced`, `MaximumBlock`, `Sending zki`, `Last Verified`, `error`, `reorg`.
