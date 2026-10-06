# CDK Example - sequence-sender + aggregator + validator

This example demonstrates running CDK with `sequence-sender` and `aggregator` as separate Docker Compose services. No ZK prover is used: the aggregator settles L1-synced batches with signatures from the bundled validator service (see `qday/docs/validator.md`).

## Prerequisites

- L1 chain with zkEVM contracts deployed (RollupManager, Bridge, GlobalExitRoot, etc.)
- L2 execution node (cdk-erigon) providing RPC at `http://127.0.0.1:8123`
- Test keystore files are bundled in `keystores/` (Hardhat test keys, password `testonly`).
  Replace them with your own keystores for production use.

## Quick Start

```bash
# 1. Prepare environment (uses the published CDK image by default)
cp env-example .env

# 2. Update contract addresses in cdk-node-config.toml

# 3. Start services
docker compose -f qday/example/docker-compose.yml up -d

# 4. Check logs
docker compose -f qday/example/docker-compose.yml logs -f
```

The aggregator runs with `SettlementBackend = "l1"` and a non-empty `Validators`
list, so the ZK prover is never started. Before calling
`verifyBatchesTrustedAggregator` on L1, the aggregator asks the validator to
sign the synced batch range. The validator independently re-checks the range
against its own L1/L2 endpoints and only signs when everything matches.

The bundled validator signs with the Hardhat test key #2
(`0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC`), injected via
`VALIDATOR_PRIVATE_KEY` in `.env`. For production, set your own key in `.env`
and update `ValidatorAddress` in `cdk-node-config.toml` to match.

The published CDK node image (`ghcr.io/qday-io/qday-cdk:qday-v0.5.4-fork12`) is pulled automatically.
To run a locally built image instead, build it with `make build-docker`
(tags the image as `cdk`) and set `CDK_NODE_IMAGE=cdk` in `.env`.

## Files

| File | Description |
|------|-------------|
| `docker-compose.yml` | Docker Compose orchestration for validator, sequence-sender and aggregator |
| `cdk-node-config.toml` | CDK node configuration file |
| `validator-config.toml` | Validator service configuration (L1/L2 RPCs, rollup id) |
| `keystores/sequencer.keystore` | Sequencer keystore (Hardhat test key, password `testonly`) |
| `keystores/aggregator.keystore` | Aggregator keystore (Hardhat test key, password `testonly`) |
| `env-example` | Environment variable template (copy to `.env`) |
| `.gitignore` | Ignores `.env`, `data/`, user-added keystores |

## Services

| Service | Container Name | Purpose |
|---------|---------------|---------|
| `validator` | `cdk-validator` | Batch confirmation service; signs synced batch ranges after independent L1/L2 checks |
| `sequence-sender` | `cdk-sequence-sender` | Sends L2 batches to L1 rollup contract |
| `aggregator` | `cdk-aggregator` | Collects validator signatures and settles batches to L1 |
