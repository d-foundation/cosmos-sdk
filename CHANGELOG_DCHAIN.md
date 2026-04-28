# DChain Customizations

This document describes the changes layered on top of upstream `cosmos-sdk` (currently rebased onto `v0.54.2`) by the dchain fork.

> In one sentence: dchain bolts a **mandatory verifiable-presentation** check onto every signed transaction, adds a **send-time hook** to the bank module so other modules can gate or track coin movements, and teaches **continuous-vesting accounts** to behave as wasm-backed abstract accounts.

## Contents

- [Features](#features)
  - [1. Verifiable Presentation (VP) on the Client](#1-verifiable-presentation-vp-on-the-client)
  - [2. Bank Hooks (BlockBeforeSend / TrackBeforeSend)](#2-bank-hooks-blockbeforesend--trackbeforesend)
  - [3. AbstractContinuousVesting + NilPubKey](#3-abstractcontinuousvesting--nilpubkey)
- [Supporting Plumbing](#supporting-plumbing)
- [Build & Dependency Changes](#build--dependency-changes)
- [File-Level Diff Summary](#file-level-diff-summary)

## Features

### 1. Verifiable Presentation (VP) on the Client

Forces every CLI-built transaction to carry a base64-encoded Verifiable Presentation that is attached to the transaction as an extension option. Transactions without a VP are rejected at the factory level.

| File | What changed |
|------|--------------|
| `client/flags/flags.go` | Adds `--verifiable-presentation` flag (`FlagVerifiablePresentation`), registered as `BytesBase64` in `AddTxFlagsToCmd` |
| `client/tx/factory.go` | `NewFactoryCLI` reads the flag, errors when empty, wraps the bytes into `*codectypes.Any` with TypeUrl `vcv.ExtensionOptionTypeUrl`, and stores it in `Factory.extOptions` |

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

### 3. AbstractContinuousVesting + NilPubKey

Lets a `ContinuousVestingAccount` act as an abstract (wasm-controlled) account in dchain's `abstractaccount` antehandler.

| File | What changed |
|------|--------------|
| `proto/cosmos/vesting/dchainv1/abstract_vesting.proto` | New proto defining `NilPubKey` |
| `x/auth/vesting/types/abstract_vesting.pb.go` | Generated gogo types |
| `api/cosmos/vesting/dchainv1/abstract_vesting.pulsar.go` | Generated pulsar types |
| `x/auth/vesting/types/abstract_vesting_account.go` | **The behavior:** see below |
| `x/auth/vesting/types/codec.go` | Registers `NilPubKey` in the codec |

The behavior file adds three things to `ContinuousVestingAccount`:

- **`IsAbstractAccount() bool`** — returns `true` when the account address is **32 bytes** (the wasm-contract address size) instead of the standard SDK 20 bytes.
- **`GetPubKey() cryptotypes.PubKey`** override — for abstract accounts returns a `NilPubKey` (so `GetSigningTxData()` doesn't panic on `nil`); otherwise delegates to the embedded base account.
- **`NilPubKey`** — a `cryptotypes.PubKey` implementation that:
  - exposes the address as `Address()`
  - returns `nil` from `Bytes()`
  - **panics** if `VerifySignature` is ever called (signatures are validated elsewhere by the abstractaccount antehandler)
  - has equality based on address bytes

## Supporting Plumbing

To enable depinject-based bank hook composition, the epochs module's keeper plumbing was tweaked to handle the keeper as a pointer, mirroring the bank change:

| File | What changed |
|------|--------------|
| `x/epochs/depinject.go` | Depinject `Provide` / `Invoke` wiring (`InvokeSetHooks`) |
| `x/epochs/keeper/keeper.go` | Keeper handled as pointer for hook-set mutability |
| `x/epochs/keeper/grpc_query.go` | `NewQuerier` takes `*Keeper` |
| `x/epochs/module.go` | Module wiring updated for pointer keeper |

## Build & Dependency Changes

| File | What changed |
|------|--------------|
| `Makefile` | proto-builder docker invocation extended to mount the host's Go module cache (`$GOMODCACHE → /go/pkg/mod`) and forward `GOPROXY` / `GOPRIVATE` env vars — required because the `d-foundation/protocol` imports are pulled from a private repo during proto generation |
| `go.mod` / `go.sum` (root + `simapp/`) | Adds `github.com/d-foundation/protocol v0.10.0` |

> **Note for v0.54.2 rebase:** the original dchain branch (off v0.53.4) also pinned `cometbft` to `v0.38.21`. v0.54.2 ships `v0.39.1`, so on the rebased branch (`dchain-54`) the cometbft pin is dropped and the upstream version is used.

## File-Level Diff Summary

Total against v0.53.4: **28 files, +1551 / -209 lines**.

| File | Lines | Category |
|------|-------|----------|
| `Makefile` | 2 | Build |
| `api/cosmos/auth/module/v1/module.pulsar.go` | 2 | Cosmetic (doc comment) |
| `api/cosmos/vesting/dchainv1/abstract_vesting.pulsar.go` | 582 | Generated (Vesting feature) |
| `client/flags/flags.go` | 3 | VP feature |
| `client/tx/factory.go` | 24 | VP feature |
| `client/v2/internal/testpbgogo/msg.proto` | 8 | Cosmetic (whitespace) |
| `client/v2/internal/testpbpulsar/msg.proto` | 8 | Cosmetic (whitespace) |
| `go.mod` / `go.sum` | 263 | Dependencies |
| `proto/cosmos/vesting/dchainv1/abstract_vesting.proto` | 18 | Vesting feature |
| `simapp/go.mod` / `simapp/go.sum` | 37 | Dependencies |
| `x/auth/vesting/types/abstract_vesting.pb.go` | 327 | Generated (Vesting feature) |
| `x/auth/vesting/types/abstract_vesting_account.go` | 78 | Vesting feature |
| `x/auth/vesting/types/codec.go` | 3 | Vesting feature |
| `x/bank/hooks_test.go` | 146 | Bank hooks |
| `x/bank/keeper/hooks.go` | 26 | Bank hooks |
| `x/bank/keeper/keeper.go` | 22 | Bank hooks |
| `x/bank/keeper/migrations.go` | 4 | Bank hooks (pointer) |
| `x/bank/keeper/msg_server.go` | 64 | Bank hooks (pointer) |
| `x/bank/keeper/send.go` | 45 | Bank hooks |
| `x/bank/module.go` | 39 | Bank hooks (depinject) |
| `x/bank/types/expected_keepers.go` | 6 | Bank hooks |
| `x/bank/types/hooks.go` | 39 | Bank hooks |
| `x/epochs/depinject.go` | 2 | Epochs plumbing |
| `x/epochs/keeper/grpc_query.go` | 4 | Epochs plumbing |
| `x/epochs/keeper/keeper.go` | 4 | Epochs plumbing |
| `x/epochs/module.go` | 4 | Epochs plumbing |
