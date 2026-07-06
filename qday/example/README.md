# CDK Example - sequence-sender + aggregator

This example demonstrates running CDK with `sequence-sender` and `aggregator` as separate Docker Compose services.

## Prerequisites

- L1 chain with zkEVM contracts deployed (RollupManager, Bridge, GlobalExitRoot, etc.)
- L2 execution node (cdk-erigon) providing RPC at `http://127.0.0.1:8123`
- ZK prover image (started by compose as the `zkevm-prover` service)
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

The published CDK node image (`ghcr.io/0xpolygon/cdk`) is pulled automatically.
To run a locally built image instead, build it with `make build-docker`
(tags the image as `cdk`) and set `CDK_NODE_IMAGE=cdk` in `.env`.

## Files

| File | Description |
|------|-------------|
| `docker-compose.yml` | Docker Compose orchestration for prover, sequence-sender and aggregator |
| `cdk-node-config.toml` | CDK node configuration file |
| `prover.config.json` | Stateless ZK prover configuration (connects to the aggregator gRPC server) |
| `keystores/sequencer.keystore` | Sequencer keystore (Hardhat test key, password `testonly`) |
| `keystores/aggregator.keystore` | Aggregator keystore (Hardhat test key, password `testonly`) |
| `env-example` | Environment variable template (copy to `.env`) |
| `.gitignore` | Ignores `.env`, `data/`, user-added keystores |

## Services

| Service | Container Name | Purpose |
|---------|---------------|---------|
| `zkevm-prover` | `cdk-zkevm-prover` | Stateless ZK prover, connects to aggregator gRPC to compute proofs |
| `sequence-sender` | `cdk-sequence-sender` | Sends L2 batches to L1 rollup contract |
| `aggregator` | `cdk-aggregator` | Generates ZK proofs and settles to L1 |
