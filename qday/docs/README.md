# CDK Docs

Documentation for deploying and operating CDK components.

## Contents

| File | Description |
|------|-------------|
| [config.md](config.md) | Configuration reference - all fields, defaults, and use cases |
| [usage.md](usage.md) | Step-by-step guide to start, monitor, and debug services |

## Component Overview

```
┌─────────────────┐     ┌──────────────────┐
│ Sequence Sender │     │    Aggregator    │
│                 │     │                  │
│ Submits L2      │     │ Generates ZK     │
│ batches to L1   │     │ proofs → L1      │
│ rollup contract │     │ settlement       │
└───────┬─────────┘     └────────┬─────────┘
        │                        │
        └────────┬───────────────┘
                 │
          ┌──────┴──────┐
          │  L1 (Ethereum) │
          └──────────────┘
```

## Quick Links

- [Example deployment](../example/README.md) - Docker Compose setup
- [Configuration reference](config.md) - All config fields documented
