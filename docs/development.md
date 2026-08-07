# pool — Development

Local setup for both halves, the branch layout that distribution depends on, and what
"releasing" means in a repo with no registry behind it.

## Prerequisites

- **Bun**, for the TypeScript half. `ts/bun.lock` is committed.
- **Go 1.24.** `go/go.mod` declares `go 1.24.0`.
- A reachable **Antenne** instance to test against anything real. There is no mock server in this
  repo.

## Setup

```sh
cd ts && bun install && bun run build
cd ../go && go mod tidy
```

## Tasks

```sh
cd ts
bun run build       # tsc — emits ts/dist/
bun run typecheck   # tsc --noEmit
```

```sh
cd go
go build ./...
go vet ./...
```

There is no `mise.toml`, no `scripts/check.sh`, no lint config and no CI here — the repo
predates the suite's chassis conventions and has not been brought onto them. There are also
**no tests in either half**, which is worth knowing before you change reconnection or offset
handling: nothing will catch you. The consumers' own suites are the only coverage —
`Sablier/apps/api/modules/antenne/agent_sessions_test.go` is the closest thing to an
integration test of this protocol.

## Committed build output

`ts/dist/` is **committed on purpose**, and `.gitignore` deliberately does not cover it. A
`github:` dependency installs whatever is in the repository — no build step runs at install
time — so an uncommitted `dist/` means every consumer installs a package with no entry point.

**Run `bun run build` and commit `ts/dist/` in the same commit as any `ts/src/` change.** A
source change without its rebuilt output ships nothing; worse, it ships silently, because the
stale `dist/` still resolves and still works.

## Branch layout

Two branches, and they are not a fork of each other's history — they are two publishing
surfaces for the same repo.

| Branch | Holds | Consumed as |
|---|---|---|
| `main` | `ts/`, `go/`, `docs/`, `README.md`, `CLAUDE.md` | `go get github.com/FacileStudio/antenne-client/go` |
| `ts` | the **contents** of `ts/` at the root | `bun add github:FacileStudio/antenne-client#ts` |

There is no `go` branch. The Go module path ends in `/go`, so the toolchain resolves the
subdirectory from the default branch directly.

`ts` exists because a `github:` dependency installs the repository root, and Bun needs to find
a `package.json` there. Its commits mirror `main`'s — the message on the current head says so
outright: `feat: emitNow — send or fail, never buffer (mirror of main 683b8b5)`.

**Check which branch you are on before you push.** A TypeScript change has to land on both, and
anything that is not the TypeScript client belongs on `main` only. Docs, in particular, live on
`main`: putting them on `ts` would ship them inside every consumer's `node_modules`.

## Releasing

There is no release. Both distribution paths resolve to a git reference, so **a push is a
publish**:

- A push to `ts` is picked up by the next `bun install` in any consumer, because `#ts` is a
  branch reference and not a version. There is no way for a consumer to pin it beyond its own
  lockfile.
- A push to `main` does not move Go consumers, which pin a pseudo-version of a specific commit
  (`v0.0.0-20260703193105-683b8b5581af` in both Sablier and Mycelium). They update deliberately,
  with `go get github.com/FacileStudio/antenne-client/go@<commit>`.

The asymmetry is worth internalizing: the same change is instant for TypeScript consumers and
opt-in for Go ones. `ts/package.json` carries `"version": "0.1.0"` and has not moved; it is not
a meaningful version number and nothing reads it.

## Keeping the two halves aligned

The protocol is defined by the two implementations agreeing, and nothing checks that they do.
When you change one, change the other in the same commit and diff them by eye. The known
asymmetries today, all of them real rather than accidental:

| Behavior | TypeScript | Go |
|---|---|---|
| Disable reconnection | `reconnect: false` | not available |
| Read the connection epoch | `currentEpoch` getter | stored, not exported |
| Handler throws or panics | reported via `onError` | not recovered; kills the read loop |
| `Listen` unsubscribe | removes the exact handler | compares handler addresses; see [api.md](api.md#listen) |
| Emit failure | `void`, errors surface via `onError` | returns `error` |

Add to that table rather than quietly closing a gap on one side only.
