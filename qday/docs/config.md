# cdk-node-config.toml - Configuration Reference

## [Common]

Common settings shared across components.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `IsValidiumMode` | bool | `false` | Enable Validium mode (uses DAC instead of calldata). Set to `true` for validium chains. |
| `ContractVersions` | string | `"banana"` | zkEVM contract version. Options: `"banana"`, `"elderberry"`. Determines which rollup fork to use. |
| `NetworkID` | uint32 | `1` | CDK network identifier used internally. |

## [Log]

Global logging configuration.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Environment` | string | `"development"` | Log environment. `"development"` adds extra debug output; `"production"` is more concise. |
| `Level` | string | `"info"` | Minimum log level. Options: `"debug"`, `"info"`, `"warn"`, `"error"`, `"fatal"`. |
| `Outputs` | []string | `["stderr"]` | Log output destinations. Supports `"stderr"`, `"stdout"`. |

## [L1Config]

L1 chain and contract addresses. These are **required** and must be filled in after deploying zkEVM contracts.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `chainId` | uint64 | `0` | **L1** chain ID (e.g., 1 for Ethereum mainnet, 11155111 for Sepolia). |
| `polygonZkEVMGlobalExitRootAddress` | address | `0x0` | Address of the Global Exit Root Manager contract on L1. |
| `polygonRollupManagerAddress` | address | `0x0` | Address of the Rollup Manager contract on L1. |
| `polTokenAddress` | address | `0x0` | Address of the POL token contract on L1. |
| `polygonZkEVMAddress` | address | `0x0` | Address of the zkEVM (rollup) contract on L1. |

## [NetworkConfig.L1]

Alternative L1 contract configuration (may duplicate [L1Config]).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `L1ChainID` | uint64 | `0` | L1 chain ID. |
| `PolAddr` | address | `0x0` | POL token address. |
| `ZkEVMAddr` | address | `0x0` | zkEVM rollup contract address. |
| `RollupManagerAddr` | address | `0x0` | Rollup Manager contract address. |
| `GlobalExitRootManagerAddr` | address | `0x0` | Global Exit Root Manager contract address. |

## [SequenceSender]

Controls how L2 batches are sent to L1.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `WaitPeriodSendSequence` | duration | `"15s"` | Interval between batch submission attempts. Higher values reduce L1 gas but increase latency. |
| `LastBatchVirtualizationTimeMaxWaitPeriod` | duration | `"10s"` | Max time to wait for the last batch to be virtualized before forcing. |
| `L1BlockTimestampMargin` | duration | `"30s"` | Safety margin added to L1 block timestamp when submitting batches. |
| `MaxTxSizeForL1` | uint64 | `131072` | Max transaction size in bytes for L1 batch submission. |
| `L2Coinbase` | address | - | L2 sequencer coinbase address (receives L2 fees). |
| `PrivateKey` | keystore | - | Sequencer keystore for signing L1 transactions. `{Path, Password}`. |
| `SequencesTxFileName` | string | `"sequencesender.json"` | Filename for tracking pending sequence transactions across restarts. |
| `GasOffset` | uint64 | `80000` | Additional gas added to the estimated gas limit for L1 transactions. |
| `WaitPeriodPurgeTxFile` | duration | `"15m"` | Interval to purge old entries from the sequence transaction file. |
| `MaxPendingTx` | uint64 | `1` | Max number of pending L1 sequence transactions at any time. |
| `MaxBatchesForL1` | uint64 | `300` | Max batches to include in a single L1 transaction (Validium mode). |
| `BlockFinality` | string | `"FinalizedBlock"` | L1 block finality level to consider safe. Options: `"LatestBlock"`, `"SafeBlock"`, `"PendingBlock"`, `"FinalizedBlock"`. |
| `RPCURL` | string | - | L2 RPC endpoint for fetching closed batches. |
| `GetBatchWaitInterval` | duration | `"10s"` | Wait interval between polling L2 for new batches. |

### [SequenceSender.EthTxManager]

L1 transaction manager for sequence sender.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `FrequencyToMonitorTxs` | duration | `"1s"` | How often to poll for transaction status changes. |
| `WaitTxToBeMined` | duration | `"2m"` | Max time to wait for a transaction to be mined. |
| `GetReceiptMaxTime` | duration | `"250ms"` | Timeout for fetching transaction receipts. |
| `GetReceiptWaitInterval` | duration | `"1s"` | Interval between receipt fetch retries. |
| `PrivateKeys` | []keystore | - | List of keystores for L1 transaction signing. |
| `ForcedGas` | uint64 | `0` | Forced gas price (0 = use estimated). |
| `GasPriceMarginFactor` | float64 | `1.0` | Multiplier applied to estimated gas price. |
| `MaxGasPriceLimit` | uint64 | `0` | Cap on gas price (0 = no cap). |
| `StoragePath` | string | - | SQLite DB path for tracking pending transactions. |
| `ReadPendingL1Txs` | bool | `false` | Read pending transactions from DB on startup. |
| `SafeStatusL1NumberOfBlocks` | uint64 | `0` | Number of confirmations before considering a tx "safe". |
| `FinalizedStatusL1NumberOfBlocks` | uint64 | `0` | Number of confirmations before considering a tx "finalized". |

### [SequenceSender.EthTxManager.Etherman]

L1 connection for EthTxManager.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `URL` | string | - | L1 RPC URL. |
| `MultiGasProvider` | bool | `false` | Use multiple gas providers for L1 gas estimation. |
| `L1ChainID` | uint64 | `0` | L1 chain ID. |
| `HTTPHeaders` | []header | `[]` | Custom HTTP headers for RPC requests. |

## [Aggregator]

Proof generation and settlement engine.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Host` | string | `"0.0.0.0"` | gRPC server bind address. |
| `Port` | int | `50081` | gRPC server port. |
| `RetryTime` | duration | `"5s"` | Wait time before retrying failed operations. |
| `VerifyProofInterval` | duration | `"10s"` | Interval to check for newly available verified proofs. |
| `ProofStatePollingInterval` | duration | `"5s"` | How often to poll proof state changes. |
| `BatchProofSanityCheckEnabled` | bool | `true` | Run sanity checks on generated batch proofs. |
| `ChainID` | uint64 | auto | L2 chain ID (auto-detected from contracts, 0 = auto). |
| `ForkId` | uint64 | `9` | zkEVM rollup fork ID. Must match the deployed contract fork. |
| `SenderAddress` | address | - | Aggregator address for submitting proofs to L1. |
| `CleanupLockedProofsInterval` | duration | `"2m"` | Interval to clean up stale locked proofs. |
| `GeneratingProofCleanupThreshold` | duration | `"10m"` | Max age before a "generating" proof is considered stale. |
| `GasOffset` | uint64 | `0` | Gas offset for L1 proof settlement transactions. |
| `RPCURL` | string | - | L2 RPC URL for fetching batches and witnesses. |
| `WitnessURL` | string | - | Witness server URL (usually same as L2 RPC). |
| `UseFullWitness` | bool | `false` | Use full witness data instead of compact. |
| `DBPath` | string | - | SQLite database path for aggregator state. |
| `SettlementBackend` | string | `"l1"` | Settlement target. `"l1"` = settle proofs directly on L1; `"agglayer"` = use AggLayer. |
| `AggLayerTxTimeout` | duration | `"5m"` | Timeout for AggLayer settlement transactions. |
| `AggLayerURL` | string | - | AggLayer API URL (required if SettlementBackend = `"agglayer"`). |
| `SyncModeOnlyEnabled` | bool | `false` | Only sync data without generating proofs (read-only mode). |
| `SequencerPrivateKey` | keystore | `{}` | Sequencer keystore for AggLayer interaction (optional). |

### [Aggregator.Log]

Per-component logging override for aggregator. Same fields as global `[Log]`.

### [Aggregator.EthTxManager]

Same structure as `[SequenceSender.EthTxManager]`, used for aggregator's L1 proof settlement transactions.

### [Aggregator.EthTxManager.Etherman]

Same structure as `[SequenceSender.EthTxManager.Etherman]`.

### [Aggregator.Synchronizer]

L1 synchronization for the aggregator to track sequenced and virtualized batches.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `[Aggregator.Synchronizer.SQLDB]` | - | - | Database config for the synchronizer. |
| `DriverName` | string | `"sqlite3"` | Database driver. `"sqlite3"` for local file. (`"postgres"` is deprecated upstream.) |
| `DataSource` | string | - | SQLite file path, e.g. `file:/app/data/aggregator_sync_db.sqlite`. |
| `[Aggregator.Synchronizer.Synchronizer]` | - | - | Sync behavior config. |
| `SyncInterval` | duration | `"10s"` | How often to sync new blocks from L1. |
| `SyncChunkSize` | uint64 | `1000` | Number of blocks to fetch per sync cycle. |
| `GenesisBlockNumber` | uint64 | `0` | Starting block for sync. |
| `SyncUpToBlock` | string | `"finalized"` | Which block to sync up to. `"latest"`, `"finalized"`. |
| `BlockFinality` | string | `"finalized"` | Block finality level. |
| `OverrideStorageCheck` | bool | `false` | Skip storage consistency checks. |
| `[Aggregator.Synchronizer.Etherman]` | - | - | L1 connection for the synchronizer. |
| `L1URL` | string | - | L1 RPC URL. |
| `ForkIDChunkSize` | uint64 | `100` | Block chunk size for fork ID detection. |
| `L1ChainID` | uint64 | `0` | L1 chain ID. |
| `ParallelBlockRequest` | bool | `false` | Fetch blocks in parallel. |
| `[Aggregator.Synchronizer.Etherman.Contracts]` | - | - | Contract addresses for sync. |
| `GlobalExitRootManagerAddr` | address | `0x0` | GER Manager address. |
| `RollupManagerAddr` | address | `0x0` | Rollup Manager address. |
| `ZkEVMAddr` | address | `0x0` | zkEVM rollup address. |
| `[Aggregator.Synchronizer.Etherman.Validium]` | - | - | Validium-specific sync config. |
| `Enabled` | bool | `false` | Enable validium mode sync. |
| `TrustedSequencerURL` | string | - | Trusted sequencer URL for DAC data. |
| `RetryOnDACErrorInterval` | duration | `"1m"` | Retry interval on DAC errors. |
| `DataSourcePriority` | []string | `["trusted","external"]` | Data source priority order. |
| `[Aggregator.Synchronizer.Etherman.Validium.Translator]` | - | - | URL translation rules for DAC access. |
| `FullMatchRules` | []rule | `[]` | Full match URL rewrite rules `{Old, New}`. |
| `[Aggregator.Synchronizer.Etherman.Validium.RateLimit]` | - | - | Rate limiting for DAC requests. |
| `NumRequests` | int | `1000` | Number of requests allowed. |
| `Interval` | duration | `"1s"` | Time window for rate limiting. |

## [ReorgDetectorL1] / [ReorgDetectorL2]

Chain reorganization detection for L1 and L2.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DBPath` | string | - | SQLite DB path for storing block history. |
| `CheckReorgsInterval` | duration | - | How often to check for reorgs. |

## [L1InfoTreeSync]

Syncs the L1 Info Tree (Global Exit Roots, L1 Info Tree leaves).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DBPath` | string | - | SQLite DB path. |
| `GlobalExitRootAddr` | address | `0x0` | Global Exit Root Manager contract address. |
| `RollupManagerAddr` | address | `0x0` | Rollup Manager contract address. |
| `URLRPCL1` | string | - | L1 RPC URL for syncing info tree events. |
| `SyncBlockChunkSize` | uint64 | `100` | Blocks to process per sync cycle. |
| `BlockFinality` | string | `"FinalizedBlock"` | Block finality level for synced data. |
| `WaitForNewBlocksPeriod` | duration | `"5s"` | Wait interval when no new blocks are found. |
| `InitialBlock` | uint64 | `0` | Starting block number for sync. |
| `RetryAfterErrorPeriod` | duration | `"10s"` | Wait time before retrying after an error. |
| `MaxRetryAttemptsAfterError` | int | `0` | Max retry attempts (0 = unlimited). |

---

## Use Cases

### sequence-sender only
Run when you only need to submit L2 batches to L1. Used by L2 operators to sequence transactions.

### aggregator only
Run when you only need to generate proofs. Requires an external prover service (gRPC). Handles proof aggregation and L1 settlement.

### sequence-sender + aggregator (combined)
Full CDK node configuration. Suitable for a single operator running both batch submission and proof generation.

### Validium mode
Set `IsValidiumMode = true` and configure `[Aggregator.Synchronizer.Etherman.Validium]` for DAC interaction.
