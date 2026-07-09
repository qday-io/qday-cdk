# CDK Usage Guide

## Prerequisites

Before starting CDK services, ensure the following upstream dependencies are available:

| Dependency | Purpose | Required by |
|------------|---------|-------------|
| L1 RPC (Ethereum) | Submit batches and proofs | sequence-sender, aggregator |
| L2 RPC (cdk-erigon) | Fetch batches and witnesses | sequence-sender, aggregator |
| Prover (gRPC) | Generate ZK proofs | aggregator |
| AggLayer (optional) | Aggregate and settle proofs | aggregator |

## Docker Image

The example uses the published CDK node image by default, configured via
`CDK_NODE_IMAGE` in `env-example`:

```
CDK_NODE_IMAGE=ghcr.io/qday-io/qday-cdk:qday-v0.5.4-fork12
```

`docker compose up` pulls it automatically — no build step required.

### Running a locally built image

Only needed for development or custom builds. The Makefile target tags the
image as `cdk` (not `cdk-node:local`):

```bash
# From project root
make build-docker          # builds image tagged `cdk`
```

Then point `CDK_NODE_IMAGE` at it in `.env`:

```
CDK_NODE_IMAGE=cdk
```

## Configuration

1. Copy the environment template:
   ```bash
   cp qday/example/env-example qday/example/.env
   ```

2. Edit `qday/example/cdk-node-config.toml`:
   - Set L1 contract addresses under `[L1Config]`
   - Set `[Etherman].URL` to your L1 RPC
   - Set `SequenceSender.EthTxManager.Etherman.URL` to your L1 RPC
   - Set `Aggregator.EthTxManager.Etherman.URL` to your L1 RPC
   - Set `Aggregator.Synchronizer.Etherman.L1URL` to your L1 RPC
   - Set `SequenceSender.RPCURL` and `Aggregator.RPCURL` to your L2 RPC
   - Set `Aggregator.WitnessURL` to your witness server URL

3. Keystore files are bundled in `qday/example/keystores/` (Hardhat test keys,
   password `testonly"). Replace them with your own keystores for production:

   - `sequencer.keystore` — For sequence sender L1 transactions
   - `aggregator.keystore` — For aggregator L1 proof settlement

   If you replace them, also update `SequenceSender.PrivateKey`,
   `Aggregator.EthTxManager.PrivateKeys`, and `Aggregator.SenderAddress` in
   `cdk-node-config.toml` to match the new keystore addresses and password.

4. The prover runs in **mock mode** by default (`runAggregatorClientMock: true` in `prover.config.json`).
   The prover image includes all necessary config files. For production,
   set `runAggregatorClientMock` to `false` in `prover.config.json` and download
   real proving files via `./download-prover-files.sh` (~115GB).

## Start Services

```bash
# Start both services
docker compose -f qday/example/docker-compose.yml up -d

# Start only sequence-sender
docker compose -f qday/example/docker-compose.yml up -d sequence-sender

# Start only aggregator
docker compose -f qday/example/docker-compose.yml up -d aggregator
```

## Stop Services

```bash
# Stop all services
docker compose -f qday/example/docker-compose.yml down

# Stop a specific service
docker compose -f qday/example/docker-compose.yml stop sequence-sender
```

## Health Check

### Check container status

```bash
docker compose -f qday/example/docker-compose.yml ps
```

Expected output:

```
NAME                    STATUS
cdk-sequence-sender     Up (healthy)
cdk-aggregator          Up (healthy)
```

### Check sequence-sender logs

```bash
docker compose -f qday/example/docker-compose.yml logs sequence-sender
```

Healthy indicators in logs:
- `"Starting application"` — Initialized successfully
- `"connected to L1"` — L1 RPC connection established
- `"sending sequence"` or similar — Actively submitting batches

Common issues:
- `"error no leaves on L1InfoTree yet and GetInitL1InfoRootMap fails"` — `InitialBlock` is set too high and missed the `InitL1InfoRootMap` event. See [config.md](config.md#initialblock-vs-genesisblocknumber). Fix: find the actual event block, update `InitialBlock`, and delete the L1InfoTreeSync DB to resync.
- `"execution reverted: ERC20: insufficient allowance"` — Sequencer hasn't approved the rollup contract to spend POL. See [config.md](config.md#pol-allowance). Fix: `cast send <POL> "approve(address,uint256)" <ROLLUP> <AMOUNT>`.
- `"Failed to create etherman"` — Check L1 RPC URL and contract addresses
- `"Required field RPCURL is empty"` — L2 RPC not configured
- Connection refused — L1/L2 nodes not running or network unreachable

### Check aggregator logs

```bash
docker compose -f qday/example/docker-compose.yml logs aggregator
```

Healthy indicators in logs:
- `"Starting application"` — Initialized successfully
- `"gRPC server listening"` — Prover connection ready
- `"Auto-discover L2ChainID"` — Contract interaction working

Common issues:
- `"Failed to create etherman"` — Check L1 RPC URL and contract addresses
- No prover connection — Prover service not running or wrong port

### Verify L1 connectivity

```bash
# Replace <L1_RPC_URL> with your actual L1 RPC endpoint
docker exec cdk-sequence-sender curl -s -X POST \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  <L1_RPC_URL>
```

Expected: Returns JSON with `"result"` field containing latest block hex.

### Verify L2 connectivity

```bash
docker exec cdk-sequence-sender curl -s -X POST \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://127.0.0.1:8123
```

Expected: Returns JSON with `"result"` field containing latest block hex.

### Check aggregator gRPC port

```bash
# Check if gRPC port is open
docker exec cdk-aggregator netstat -tlnp | grep 50081
```

Expected: Shows port 50081 in LISTEN state.

### Check data persistence

```bash
ls qday/example/data/sequence-sender/
ls qday/example/data/aggregator/
```

Expected: SQLite database files present (e.g., `ethtxmanager.db`, `aggregator.sqlite`,
`aggregator_sync_db.sqlite`).

## Debugging

### Increase log verbosity

Set `CDK_LOG_LEVEL=debug` in `.env` and restart:

```bash
echo "CDK_LOG_LEVEL=debug" >> qday/example/.env
docker compose -f qday/example/docker-compose.yml up -d
```

### Access container shell

```bash
docker exec -it cdk-sequence-sender /bin/sh
docker exec -it cdk-aggregator /bin/sh
```

### Save resolved configuration

The CDK node can save its merged configuration for debugging:

```bash
docker exec cdk-sequence-sender cdk-node run \
  --cfg /app/config/cdk-node-config.toml \
  --save-config-path /app/data/ \
  --components sequence-sender,aggregator
```

This writes `cdk-node-config.toml` with all defaults resolved to `/app/data/`.

### View EthTxManager state

The EthTxManager SQLite database tracks pending L1 transactions:

```bash
docker exec cdk-sequence-sender sqlite3 /app/data/ethtxmanager.db ".tables"
docker exec cdk-sequence-sender sqlite3 /app/data/ethtxmanager.db "SELECT * FROM monitored_txs;"
```
