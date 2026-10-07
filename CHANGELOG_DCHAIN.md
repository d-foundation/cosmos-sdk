# DChain Customizations

This document describes the changes layered on top of upstream `cosmos-sdk` (currently rebased onto `v0.54.3`) by the dchain fork.

> In one sentence: dchain bolts a **mandatory verifiable-presentation** check onto every signed transaction and adds a **send-time hook** to the bank module so other modules can gate or track coin movements.

## Contents

- [Features](#features)
  - [1. Verifiable Presentation (VP) on the Client](#1-verifiable-presentation-vp-on-the-client)
  - [2. Bank Hooks (BlockBeforeSend / TrackBeforeSend)](#2-bank-hooks-blockbeforesend--trackbeforesend)
- [Removed in v0.54.3 Rebase](#removed-in-v0543-rebase)
- [Build & Dependency Changes](#build--dependency-changes)
- [File-Level Diff Summary](#file-level-diff-summary)

## Features

### 1. Verifiable Presentation (VP) on the Client

Allows every CLI-built transaction to carry a base64-encoded Verifiable Presentation attached as a transaction extension option. When the `--verifiable-presentation` flag is provided, the factory wraps the bytes into a `*codectypes.Any` extension option and stores it in `Factory.extOptions`; when the flag is absent the factory builds the transaction without the extension option. Enforcement of the VP's presence happens on-chain via the vcv ante handler, not in the factory.

| File | What changed |
|------|--------------|
| `client/flags/flags.go` | Adds `--verifiable-presentation` flag (`FlagVerifiablePresentation`), registered as `BytesBase64` in `AddTxFlagsToCmd` |
| `client/tx/factory.go` | `NewFactoryCLI` reads the flag; when non-empty, wraps the bytes into `*codectypes.Any` with TypeUrl `vcv.ExtensionOptionTypeUrl` and stores it in `Factory.extOptions`; when empty, `extOptions` is nil and the tx is built without the extension option (on-chain vcv ante handler enforces presence) |

New runtime dependency: `github.com/d-foundation/protocol v0.10.0` (provides the `vcv` package and `VerifiablePresentation` proto type).

### 2. Bank Hooks (BlockBeforeSend / TrackBeforeSend)

Lets other modules veto or observe coin movements **before** they happen — used for compliance gating in dchain.

**New types:**

| File | Purpose |
|------|---------|
| `x/bank/types/hooks.go` | `BankHooks` interface (`BlockBeforeSend`, `TrackBeforeSend`) and `MultiBankHooks` aggregator that fans out to every registered hook |
| `x/bank/keeper/hooks.go` | Keeper-side dispatch: `BlockBeforeSend` returns the first non-nil error; `TrackBeforeSend` is fire-and-forget |
| `x/bank/hooks_test.go` | Unit tests covering the hook flow |

**Wiring changes:**

| File | What changed |
|------|--------------|
| `x/bank/keeper/send.go` | `BaseSendKeeper` gains a `hooks types.BankHooks` field plus a `SetHooks` setter (panics on double-set). `SendCoins` is split: the public method calls `BlockBeforeSend` first; the private `sendCoins` performs the move and calls `TrackBeforeSend`. New `SendCoinsWithoutBlockHook` exposes the bypass path. `InputOutputCoins` now invokes both hooks per output. |
| `x/bank/keeper/keeper.go` | Wires the hooks through `BaseKeeper` |
| `x/bank/module.go` | Depinject wiring so external modules can supply hooks |
| `x/bank/keeper/msg_server.go` | `msgServer` holds `*BaseKeeper` (pointer) instead of an embedded value, so `SetHooks` mutations are visible to all consumers |
| `x/bank/keeper/migrations.go` | Converted to take `*BaseKeeper` |
| `x/bank/types/expected_keepers.go` | Exposes the hook surface in the expected-keepers contract |

> **Why pointer keeper?** `SetHooks` mutates the keeper. With an embedded value, the mutation would only affect a local copy. Switching to a pointer ensures every component (msgServer, migrator, depinject consumers) sees the same hook set.

## Removed in v0.54.3 Rebase

Two customizations carried through earlier rebases were dropped when moving to `v0.54.3`:

- **AbstractContinuousVesting + NilPubKey** (`proto/cosmos/vesting/dchainv1/`, `api/cosmos/vesting/dchainv1/`, `x/auth/vesting/types/abstract_vesting*`, codec registration). It existed solely to let `ContinuousVestingAccount` act as a wasm-controlled abstract account for the cosmwasm `abstractaccount` antehandler — cosmwasm has been fully removed from dchain, and the Protocol repo has zero references to these types. `x/auth/vesting` is now identical to upstream.
- **Epochs pointer-keeper plumbing** (`x/epochs/depinject.go`, `x/epochs/keeper/keeper.go`, `x/epochs/keeper/grpc_query.go`, `x/epochs/module.go`). Upstream v0.54 already ships the equivalent wiring (`InvokeSetHooks`, pointer keeper handling, `EpochHooksWrapper`), so the fork patch was redundant. `x/epochs` is now identical to upstream.

## Build & Dependency Changes

| File | What changed |
|------|--------------|
| `Makefile` | proto-builder docker invocation extended to mount the host's Go module cache (`$GOMODCACHE → /go/pkg/mod`) and forward `GOPROXY` / `GOPRIVATE` env vars — required because the `d-foundation/protocol` imports are pulled from a private repo during proto generation |
| `go.mod` / `go.sum` (root + `simapp/`) | Adds `github.com/d-foundation/protocol v0.10.0` |
| `tests/go.mod` / `tests/go.sum` | `go mod tidy` pulls in `github.com/99designs/keyring v1.2.2` (plus `github.com/DataDog/datadog-go` and `github.com/desertbit/timer` as transitive indirects) required by the `d-foundation/protocol` dep graph |

> **Note for v0.54.2 rebase:** the original dchain branch (off v0.53.4) also pinned `cometbft` to `v0.38.21`. v0.54.2 ships `v0.39.1`, so on the rebased branch the cometbft pin is dropped and the upstream version is used.

## File-Level Diff Summary

Total against v0.54.3: **28 files** (including this changelog).

| File | Lines | Category |
|------|-------|----------|
| `CHANGELOG_DCHAIN.md` | — | Docs (this file) |
| `Makefile` | 2 | Build |
| `client/flags/flags.go` | 3 | VP feature |
| `client/grpc_query_test.go` | 2 | Bank hooks (pointer, test adjustment) |
| `client/tx/factory.go` | 24 | VP feature |
| `go.mod` / `go.sum` | 20 | Dependencies |
| `simapp/app.go` | 2 | Bank hooks (pointer) |
| `simapp/go.mod` / `simapp/go.sum` | 18 | Dependencies |
| `store/**/*.pb.go` (5 files) | 242 | Generated (regen noise from fork proto-gen; no functional change) |
| `tests/integration/blockstm/blockstm_test.go` | 4 | Bank hooks (pointer, test adjustment) |
| `tests/integration/distribution/keeper/msg_server_test.go` | 3 | Bank hooks (pointer, test adjustment) |
| `tests/integration/staking/keeper/determinstic_test.go` | 2 | Bank hooks (pointer, test adjustment) |
| `x/bank/hooks_test.go` | 146 | Bank hooks |
| `x/bank/keeper/hooks.go` | 26 | Bank hooks |
| `x/bank/keeper/keeper.go` | 22 | Bank hooks |
| `x/bank/keeper/keeper_test.go` | 6 | Bank hooks (pointer, test adjustment) |
| `x/bank/keeper/migrations.go` | 4 | Bank hooks (pointer) |
| `x/bank/keeper/msg_server.go` | 60 | Bank hooks (pointer) |
| `x/bank/keeper/send.go` | 45 | Bank hooks |
| `x/bank/module.go` | 39 | Bank hooks (depinject) |
| `x/bank/types/expected_keepers.go` | 6 | Bank hooks |
| `x/bank/types/hooks.go` | 39 | Bank hooks |
