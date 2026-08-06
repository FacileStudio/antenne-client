# pool — Architecture

The wire protocol between a Facile app and a Nook instance, how the client keeps the
connection alive, and how the two language halves are distributed.

## Runtime topology

```
Opus (TS)      ─┐
Sablier (Go)   ─┤
Jardin (Go)    ─┼──▶ POST <instance>/api/pool/register  ──▶ Nook
                │        └── { token, app_id }
                │
                └──▶ WS  <instance>/api/pool/ws?token=…  ◀──▶ Nook Pool
                            client ──▶ subscribe, event, ack, ping
                            client ◀── welcome, event, error, pong
```

No app connects to another app. Every edge in this picture terminates at Nook, which is the
whole point of the architecture: adding an app adds one edge, not N.

## Connection lifecycle

`connect()` / `Connect(ctx)` is two steps, in order:

1. **Register.** `POST <instance>/api/pool/register` with a JSON body of `app`, `instance_id`,
   `secret` and `events`. A non-200 fails the connect with the status and the response body
   included in the message. A 200 returns `{ "token": ..., "app_id": ... }`, both kept in
   memory only.
2. **Open the socket.** The WebSocket URL is the instance URL with `http` replaced by `ws` —
   the first occurrence only, so `https://` becomes `wss://` — plus
   `/api/pool/ws?token=<url-encoded token>`. The token travels in the query string, which is
   why every Facile API redacts `token` from its request logs.

On open the client immediately sends one `subscribe` message, then flushes anything queued by
`emit` while it was down.

Both steps run again on every reconnect attempt, so an expired token is not a failure mode —
the client always registers fresh.

## Protocol

Every message is JSON over a text WebSocket frame.

**Client to server**

| Type | Fields | When |
|---|---|---|
| `subscribe` | `channels: { <channel>: { last_offset } }` | Once, immediately on open |
| `event` | `id`, `channel`, `payload`, `timestamp` | On `emit` / `emitNow` |
| `ack` | `channel`, `offset` | After every inbound event is dispatched |
| `ping` | — | Every 30 s, in both clients |

**Server to client**

| Type | Fields the client reads | Effect |
|---|---|---|
| `welcome` | `epoch` | Stored; readable as `currentEpoch` / not exported in Go |
| `event` | `id`, `channel`, `offset`, `timestamp`, `sender`, `payload` | Dispatched, then acked |
| `error` | `code`, `message` | Surfaced through the error callback |
| `pong` | — | Ignored |

Underneath those, the server sends a WebSocket **ping control frame** every 20 seconds and
drops a connection that has been silent for 60. Neither client implements that handshake:
every WebSocket stack answers a ping with a pong on its own, below the API. It exists because
a pool socket carries nothing at all while a suite is quiet, and the proxies in front of a
deployed Nook cull an idle connection — Cloudflare at around 100 seconds. The clients' own
30-second `ping` is the same defence from the other side, for a pool server that predates the
control-frame keepalive.

`id` on an outbound event is a fresh random identifier — `crypto.randomUUID()` in TypeScript,
16 random bytes hex-encoded in Go. `timestamp` is epoch milliseconds. Anything the server
sends that is not one of the four types above is silently ignored, including the `subscribed`
acknowledgement.

## Identity and echo filtering

The pool identity is `app` alone, or `app:instance_id` when `instance_id` is configured. It is
the `sender` on every event the app emits, and it lets several instances of the same app share
one pool without them being confused for each other.

On every inbound event the client compares `sender` against its own identity and **drops the
message before dispatching it**. This is why an app can list the same channel in both `emit`
and `listen` — which every consumer does — without looping its own writes back through its own
handlers.

## Offsets and replay

Each event carries a per-channel monotonic `offset`. The client records the latest offset per
channel as it dispatches, and sends the recorded value as `last_offset` in the `subscribe`
message on every connect. A channel never seen before starts at `0`.

The subscribe set is the union of `events.listen` from the configuration and every channel with
a registered handler, so a handler attached after `connect()` is still subscribed on the next
reconnect — but not on the current connection. Register handlers before connecting, or accept
that the subscription lands one reconnect later.

Offsets live in memory. A process restart replays from `0` on every channel, which is why the
event contract is built around an `idempotency_key`.

## Reconnection

Triggered by a socket close: an unclean close in TypeScript (`event.wasClean === false`), any
read error in Go. Backoff is `min(500ms * 2^attempt, 30s)` plus up to 500 ms of jitter, capped
at `maxReconnectAttempts` / `WithMaxReconnect`, default 20. The attempt counter resets to zero
on a successful open. `disconnect()` / `Disconnect()` clears the reconnect flag first, so a
deliberate shutdown does not race a reconnect.

At the default settings the backoff reaches its 30 s ceiling around attempt 6, so 20 attempts is
roughly eight minutes of retrying before the client gives up and reports through the error
callback. Nothing restarts it after that.

## Buffering, and why `emitNow` exists

`emit` / `Emit` sends when connected and otherwise appends the serialized message to an
in-memory queue, flushed on the next successful open. It reports success either way.

That is wrong for any caller with its own durability. An outbox drainer marks a row as sent
when the emit returns, so buffering would let it mark rows sent while the payload only sits in
RAM — and a crash loses both the row's pending state and the event. `emitNow` / `EmitNow`
sends immediately or reports failure and **never buffers**: `false` in TypeScript,
`ErrNotConnected` in Go. Use it whenever the caller tracks delivery itself.

The queue is unbounded. A long outage with a busy emitter grows it without limit.

## Cross-repo consumption

This is the suite's only real cross-repo code dependency, and the precedent for every future
shared package. Neither half goes through a registry.

**TypeScript — a git branch, not a registry.**

```json
"@facile/pool": "github:FacileStudio/pool#ts"
```

That is Opus's `apps/api/package.json`, verbatim. The `ts` branch holds the **contents** of
`ts/` at its root — `package.json`, `src/`, `dist/`, `tsconfig.json`, `bun.lock` — because a
`github:` dependency installs the repository root and Bun would otherwise find no
`package.json`. `dist/` is committed for the same reason: nothing runs `tsc` at install time.

The consequence to keep in mind: `#ts` is a **moving branch reference**, not a version. There
is no lockfile-independent way to pin it, so an install after a push to `ts` picks up new code.
Changes to the TypeScript client are effectively published the moment they land on that branch.

**Go — a commit, from the default branch.**

```
require github.com/FacileStudio/pool/go v0.0.0-20260703193105-683b8b5581af
```

The module path includes the `go/` subdirectory, so the Go toolchain resolves it from `main` —
there is no `go` branch. There are no semver tags either, so consumers pin the pseudo-version
of a specific commit. Sablier and Jardin are both on `683b8b5`. Updating is
`go get github.com/FacileStudio/pool/go@<commit>` and a review of the diff, not a version bump.

Both distribution paths mean **a push to `main` or `ts` is a release**. See
[development.md](development.md#releasing).

## Consumers

| App | Language | Where the client is wired |
|---|---|---|
| Opus | TypeScript | `apps/api/src/nook-pool/` |
| Sablier | Go | `apps/api/modules/nookpool/` |
| Jardin | Go | `internal/server/emitter.go` |

Each keeps a `nook.yaml` at its repo root documenting the channels it uses, even though all
three build their pool config inline in code. The file is documentation that the code is
expected to match, not the source of truth.
