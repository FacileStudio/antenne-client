# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-10

The first tagged version. This module now owns the whole wire contract: the connection, the
event envelope, the channel catalog and a durable outbox. No existing exported identifier
changed.

### Added

- `go/envelope`: the event envelope, absorbed from `github.com/FacileStudio/enveloppe/go` with
  identical JSON keys. New `App` constants: `AppAgenda`, `AppJournal`, `AppCourrier`,
  `AppNuage`, `AppBulle`, `AppAntenne`. `EventVersion` stays 1, since the change is additive.
- `go/envelope/testdata/`: one golden JSON fixture per payload type, checked byte for byte by
  the Go test and by the TypeScript test. This is the first parity proof between the halves.
- `go/catalog`: a `Channel` type, one constant per channel the suite emits or listens to (25),
  and `All()`.
- `go/outbox`: a durable Postgres outbox on `database/sql`. `Migration` holds the goose SQL,
  `Enqueue` queues inside the caller's transaction, and `NewDrainer(...).Run` sends rows in id
  order with `FOR UPDATE SKIP LOCKED`, stops at the first failure, backs off, and prunes sent
  rows after 7 days.
- `ConfigFromEnv(app)` and `ErrNotConfigured` in the root Go package. It reads `ANTENNE_URL`,
  or its synonym `ANTENNE_INSTANCE`; `ANTENNE_URL` wins when both are set.
- TypeScript: `envelope.ts` (the `Facile*` types and `FACILE_EVENT_VERSION`) and `catalog.ts`
  (`Channels`, `Channel`, `allChannels()`), re-exported from `@facile/antenne`.
- `bun run test` (typecheck, then `bun test`), `scripts/check.sh`, `filet.yml` and
  `lefthook.yml`.

### Changed

- `go/go.mod` declares `go 1.26`, which the `tronc/testdb` and `tronc/migrate` test
  dependencies require. Every consumer is already on 1.26.
- `ts/package.json` carries `"version": "1.0.0"`. Releases are tagged `go/vX.Y.Z` for the Go
  module and `vX.Y.Z` for the repository.

[Unreleased]: https://github.com/FacileStudio/antenne-client/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/FacileStudio/antenne-client/releases/tag/v1.0.0
