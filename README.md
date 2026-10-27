# OmniConnector

A unified platform for connecting, managing, and operating external services, development platforms, servers, and infrastructure from a single environment.

## Overview

OmniConnector provides a centralized integration layer that handles connections, authentication, capabilities, permissions, and operation execution — connecting services like GitHub, Vercel, SSH servers, databases, and other providers through a standardized interface.

## Plan and Status

- [`docs/PLAN.md`](docs/PLAN.md) — the product, architecture, and phased development plan (the project's governing goal)
- [`docs/STATUS.md`](docs/STATUS.md) — what is done, what is next, and interim decisions

## Client SDK

```bash
npm install @hilbras/omniconnector
```

TypeScript client for the Kernel API — see [`packages/client-sdk`](packages/client-sdk/README.md).

## Monorepo Structure

```
OmniConnector/
├── apps/                    # Desktop App and Web Dashboard
├── services/                # Backend services (kernel-api, local-agent)
├── packages/                # Shared packages (kernel-runtime, sdk, contracts, client-sdk, ui)
├── integrations/            # Service-specific integration modules
├── tests/                   # Test suites (contract, integration, security, e2e)
├── docs/                    # Documentation
├── deployment/              # Docker, CI/CD, deployment configs
├── scripts/                 # Build and utility scripts
└── Makefile                 # Build orchestration
```

## Development

Prerequisites:
- Go 1.22+
- Docker
- Node.js 22.12+ (client SDK development; the published package runs on 20.3+)

```bash
make build         # Build the kernel API
make test          # Run kernel runtime tests
make test-all      # Run every module's tests
make test-race     # Run every module's tests with the race detector
make lint          # go vet every module
make format-check  # Fail on unformatted files
make run-dev       # Run the API locally with the test integration
```

## License

See [LICENSE](LICENSE).
