# antenne-client

Client library for connecting Facile Suite apps to the Antenne event bus. Ships two
implementations of one protocol: TypeScript (`@facile/antenne`) and Go.

Apps do not talk to each other. Each one talks to Antenne, and `antenne-client` is the wire.

## What it does

- Registers an app with an Antenne instance over HTTP, then holds a WebSocket to it
- Emits events on named channels and dispatches inbound ones to per-channel handlers
- Filters out an app's own events, so emitting and listening on the same channel is safe
- Tracks a per-channel offset and replays from it on reconnect
- Reconnects automatically with exponential backoff and jitter, re-registering each attempt
- Buffers emits while disconnected, with `emitNow` / `EmitNow` for callers that need to know
- Owns the event envelope (`go/envelope`, `envelope.ts`) and the channel catalog
  (`go/catalog`, `catalog.ts`), checked against shared golden fixtures from both halves
- Ships a durable Postgres outbox for Go apps (`go/outbox`), so an event commits or rolls back
  with the business write that caused it

## Stack

| Layer | Tech |
|---|---|
| Runtime | TypeScript 5, ES2020/NodeNext, `yaml` ^2.7.1, the platform `WebSocket` and `fetch` |
| Runtime | Go 1.26, `github.com/gorilla/websocket` v1.5.3, `gopkg.in/yaml.v3` v3.0.1, `database/sql` for the outbox |
| Tests | `go test -race`, `tronc/testdb` against real Postgres, `bun test` |

## Install

There is no npm registry. Both halves install straight from GitHub. The Go module is tagged
`go/vX.Y.Z`; the TypeScript package follows the `ts` branch. See
[docs/architecture.md](docs/architecture.md#cross-repo-consumption).

```sh
bun add github:FacileStudio/antenne-client#ts
```

```ts
import { AntenneClient } from "@facile/antenne";

const pool = new AntenneClient({
  config: {
    app: "opus",
    instance: "http://localhost:3400",
    secret: "shared-secret",
    events: {
      emit: ["project.created"],
      listen: ["project.created"],
    },
  },
});

await pool.connect();

pool.emit("project.created", { facile_id: "fac_abc123", name: "Acme" });

pool.listen("project.created", (payload, meta) => {
  console.log(`project.created from ${meta.sender} at offset ${meta.offset}`, payload);
});
```

```sh
go get github.com/FacileStudio/antenne-client/go@v1.0.0
```

```go
import pool "github.com/FacileStudio/antenne-client/go"

client := pool.NewClient(&pool.Config{
	App:      "sablier",
	Instance: "http://localhost:3400",
	Secret:   "shared-secret",
	Events:   pool.EventConfig{Emit: []string{"project.created"}, Listen: []string{"project.created"}},
})

if err := client.Connect(ctx); err != nil {
	return err
}
defer client.Disconnect()

_ = client.Emit("project.created", payload)
client.Listen("project.created", func(payload json.RawMessage, meta pool.EventMeta) {
	// handle
})
```

## Envelope and catalog

Wrap every payload in the shared envelope and name channels from the catalog, never as string
literals:

```go
import (
	"github.com/FacileStudio/antenne-client/go/catalog"
	"github.com/FacileStudio/antenne-client/go/envelope"
)

evt := envelope.Event[envelope.Task]{
	Version: envelope.EventVersion, App: envelope.AppOpus,
	Object: envelope.ObjectTask, Action: envelope.ActionCreated,
	FacileID: task.FacileID, Payload: task,
	Timestamp: now, IdempotencyKey: key,
}
_ = client.EmitNow(string(catalog.TaskCreated), evt)
```

```ts
import { Channels, type FacileEvent, type FacileTask } from "@facile/antenne";

pool.listen<FacileEvent<FacileTask>>(Channels.TaskCreated, (evt) => console.log(evt.payload.name));
```

`go/envelope` replaces `github.com/FacileStudio/enveloppe/go`. The JSON keys are identical, so
switching the import is the whole migration.

## Durable outbox

`Emit` buffers in RAM and `EmitNow` fails while disconnected. For an event that must not be
lost, enqueue it in the business transaction and let a drainer send it:

1. Copy `outbox.Migration` verbatim into your migrations directory under the next number,
   for example `apps/api/migrations/00009_antenne_outbox.sql`. It holds both `-- +goose Up`
   and `-- +goose Down`. It uses `CREATE TABLE` without `IF NOT EXISTS`, so an older
   `antenne_outbox` table fails the migration loudly instead of being silently kept.
2. Inside the transaction that writes the business row:
   `outbox.Enqueue(ctx, tx, catalog.TaskCreated, evt, evt.IdempotencyKey)`. A duplicate key
   returns `outbox.ErrDuplicate` and leaves the transaction usable.
3. At boot, after `client.Connect`:
   `go outbox.NewDrainer(sqlDB, client.EmitNow).Run(ctx)`. A GORM app passes `gormDB.DB()`.

Delivery is at least once: a crash between the send and the commit resends the row. Consumers
deduplicate on `idempotency_key`. Postgres stores the payload as `jsonb`, which reorders object
keys, so the bytes on the wire can differ from the bytes enqueued. Several drainers may run
against one table; no row is sent twice.

## Configuration

Configuration is passed inline or loaded from a `antenne.yaml` at the project root.

| Key | Required | What it does |
|---|---|---|
| `app` | yes | App name; the pool identity and the sender on every event it emits |
| `instance` | yes | Antenne base URL, `http://` or `https://`; the WebSocket URL is derived from it |
| `secret` | yes | Shared secret sent at registration |
| `instance_id` | no | Makes the identity `app:instance_id`, so several instances share one pool |
| `events.emit` | no | Documented emit channels |
| `events.listen` | no | Channels subscribed on connect, before any handler is registered |

`ConfigFromEnv(app)` builds the Go `Config` from `ANTENNE_URL` (or its synonym
`ANTENNE_INSTANCE`), `ANTENNE_SECRET`, `ANTENNE_APP` and `ANTENNE_INSTANCE_ID`, and returns
`ErrNotConfigured` when no URL is set.

Full reference, including `ANTENNE_CONFIG_PATH` and the `loadConfig` / `loadManifest` split:
[docs/configuration.md](docs/configuration.md).

## Structure

```
ts/    src/index.ts is the client, envelope.ts and catalog.ts the contract; dist/ is committed
go/    antenne.go is the client; envelope/, catalog/ and outbox/ are the contract packages
docs/  architecture, configuration, development, API reference
```

## Documentation

| Doc | What's in it |
|---|---|
| [Architecture](docs/architecture.md) | Protocol, connection lifecycle, offsets, cross-repo consumption |
| [Configuration](docs/configuration.md) | Every config key and environment variable |
| [Development](docs/development.md) | Local setup, the build, the branch layout, releasing |
| [API](docs/api.md) | Every exported symbol, in both languages |
| [Changelog](CHANGELOG.md) | What changed in each version |

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.

## License

antenne-client is licensed under the [MIT License](LICENSE).
