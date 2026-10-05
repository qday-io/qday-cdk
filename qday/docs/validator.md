# Validator Service

When `[Aggregator].SettlementBackend` is `"l1"` and `[Aggregator].Validators` is not empty, the aggregator does not start the ZK prover. Before it calls the existing L1 function `verifyBatchesTrustedAggregator`, it asks every configured validator to sign the L1-synced batch range.

A validator is a separate HTTP service. It holds one secp256k1 key, checks the batch on its own, and returns a signature. The address of that key is `ValidatorAddress` in the aggregator config. The aggregator accepts the response only when the returned `signer` is that address and the signature recovers to it.

Settlement needs at least `(len(Validators)+1)/2` signatures. They are packed into the existing `proof` argument (`bytes32[24]`): one word for the count, then four words per signature (`signer`, `r`, `s`, `v`). That argument holds at most five signatures.

```toml
Validators = [
  {URL = "http://validator-1:9001", ValidatorAddress = "0xabc..."},
  {URL = "http://validator-2:9001", ValidatorAddress = "0xdef..."},
]
```

The aggregator POSTs to `URL` exactly as configured. There is no extra path.

## What the service must do

| Function | Why |
|----------|-----|
| Confirm API | Accept the aggregator's batch payload and return `{signer, signature}` |
| Key custody | Keep the signing key local. `signer` is the address of that key |
| Independent checks | Re-read L1 and L2 and refuse when the payload does not match |
| Sign the digest | Ethereum personal-sign of the six settlement fields, in the order below |
| Refuse safely | Non-200, or a body the aggregator cannot decode, fails the whole round |
| Replay control | Do not sign a range that is already verified or that overlaps a signature still in flight |

The aggregator retries a failed round after `VerifyProofInterval` (example config: `10s`). It does not stop after a fixed number of attempts.

## 1. Confirm API

`POST` the configured URL.

Request headers: `Content-Type: application/json`.

The aggregator waits 30 seconds and reads at most 1 MiB of the body. A status other than `200` fails the round. The body of a failure is only logged.

### Request

```json
{
  "rollupId": 1,
  "initNumBatch": 4,
  "finalNewBatch": 7,
  "newLocalExitRoot": "0x1111…",
  "newStateRoot": "0x2222…",
  "accInputHash": "0x3333…"
}
```

| Field | Meaning |
|-------|---------|
| `rollupId` | Rollup id from the L1 rollup manager |
| `initNumBatch` | Batch already verified on L1. The new range starts at `initNumBatch + 1` |
| `finalNewBatch` | Last virtual batch in this settlement. One signature covers the whole range |
| `newLocalExitRoot` | Local exit root of `finalNewBatch` from the L2 node |
| `newStateRoot` | State root of `finalNewBatch` from the L2 node |
| `accInputHash` | Accumulated input hash stored on L1 when `finalNewBatch` was sequenced |

Hashes are `0x`-prefixed 32-byte hex strings.

### Success response

`200` and JSON:

```json
{
  "signer": "0xabc…",
  "signature": "0x…"
}
```

| Field | Rule |
|-------|------|
| `signer` | 20-byte hex address. Must equal this validator's `ValidatorAddress` |
| `signature` | 65-byte hex (`r` \|\| `s` \|\| `v`). `v` may be `0`/`1` or `27`/`28` |

The aggregator recovers the signer from the signature. Recovery must equal `signer`, and `signer` must equal `ValidatorAddress`. A valid signature from any other key is rejected.

## 2. Key custody

- One service instance signs with one key. That key's address is the `ValidatorAddress` configured for its URL.
- The private key stays on the validator. The HTTP response contains only the address and the signature.
- The aggregator does not send a key, and it does not accept a `signer` that merely matches the signature if that address is not the configured one.

## 3. Checks before signing

The aggregator already compares `BatchL2Data` for every batch from `initNumBatch + 1` through `finalNewBatch` (batch `1` is skipped; it is the injected batch). The validator must repeat the checks against its own L1 and L2 endpoints. Sign only when all of them pass.

1. `rollupId` is the rollup this validator is responsible for.
2. `finalNewBatch` > `initNumBatch`.
3. On L1, the rollup's latest verified batch is `initNumBatch`. If L1 has already moved past it, refuse. The aggregator will send a newer range on a later round.
4. On L1, `finalNewBatch` has been sequenced, and its accumulated input hash equals `accInputHash`.
5. The L1 synchronizer data for every batch `initNumBatch + 1 … finalNewBatch` exists. For each of those batches except batch `1`, `BatchL2Data` stored from L1 equals the L2 RPC batch data.
6. The L2 batch `finalNewBatch` has `stateRoot == newStateRoot` and `localExitRoot == newLocalExitRoot`. An empty state root means the batch is not ready.
7. The signing key's address equals the `signer` that will be returned.

If any check fails, respond with a non-200 status and do not return a signature. The aggregator will not submit that range to L1.

## 4. Digest and signature

Sign the Ethereum personal-sign hash of the ABI encoding of the six fields, in this order and these types:

```text
abi.encode(
  uint32 rollupId,
  uint64 initNumBatch,
  uint64 finalNewBatch,
  bytes32 newLocalExitRoot,
  bytes32 newStateRoot,
  bytes32 accInputHash
)
```

```text
digest    = keccak256(abi.encode(...))
ethHash   = keccak256("\x19Ethereum Signed Message:\n32" || digest)
signature = secp256k1_sign(ethHash)   // 65 bytes, recoverable
```

`eth_sign` / `personal_sign` of the 32-byte `digest` produces the same `ethHash`. Return `signer` as the address of the key that signed, and `signature` as `0x` plus 130 hex characters.

Reference implementation: `aggregator/validatorsign.Digest`, `EthSignedHash`, and `Verify`.

## 5. Replay control

- Refuse a request whose `finalNewBatch` is less than or equal to the verified batch on L1.
- If this validator has already signed the same `(rollupId, initNumBatch, finalNewBatch, newStateRoot, accInputHash)` and that settlement is not yet verified, return the same signature again. The aggregator retries the same range until L1 shows it as verified.
- If any signed field changes, treat it as a new payload and run the checks again. Do not reuse the old signature.

## 6. Operations

These are not called by the aggregator. Run them so the service can be operated.

| Function | Behavior |
|----------|----------|
| Health | Report process up, L1 RPC reachable, L2 RPC reachable, and the signing key loaded. Do not put this on the confirm URL |
| Logs | Log each accept and each refusal with `rollupId`, `initNumBatch`, `finalNewBatch`, and the reason. Do not log the private key |
| Config | `URL` this process serves, the public `ValidatorAddress`, L1 RPC, L2 RPC, and the rollup id it will sign for |

## What the aggregator does with the signature

After every configured validator returns a matching signature, the aggregator calls the unchanged L1 function:

```text
verifyBatchesTrustedAggregator(
  rollupID,
  pendingStateNum = 0,
  initNumBatch,
  finalNewBatch,
  newLocalExitRoot,
  newStateRoot,
  beneficiary,          // Aggregator SenderAddress
  proof                 // bytes32[24], packed signatures
)
```

`accInputHash` is not a separate argument of that function. It is covered only by the signature.
