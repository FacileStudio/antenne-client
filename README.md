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

## Stack

| Layer | Tech |
|---|---|
| Runtime | TypeScript 5, ES2020/NodeNext, `yaml` ^2.7.1, the platform `WebSocket` and `fetch` |
| Runtime | Go 1.24, `github.com/gorilla/websocket` v1.5.3, `gopkg.in/yaml.v3` v3.0.1 |

## Install

There is no npm registry and no Go module proxy tag behind this. Both halves are consumed
straight from GitHub — see [docs/architecture.md](docs/architecture.md#cross-repo-consumption).

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
go get github.com/FacileStudio/antenne-client/go
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

Full reference, including `ANTENNE_CONFIG_PATH` and the `loadConfig` / `loadManifest` split:
[docs/configuration.md](docs/configuration.md).

## Structure

```
ts/    src/index.ts is the whole TypeScript client; dist/ is committed output
go/    pool.go is the whole Go client
docs/  architecture, configuration, development, API reference
```

## Documentation

| Doc | What's in it |
|---|---|
| [Architecture](docs/architecture.md) | Protocol, connection lifecycle, offsets, cross-repo consumption |
| [Configuration](docs/configuration.md) | Every config key and environment variable |
| [Development](docs/development.md) | Local setup, the build, the branch layout, releasing |
| [API](docs/api.md) | Every exported symbol, in both languages |

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
