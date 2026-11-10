# pool — API

The complete exported surface of both implementations. Protocol behavior lives in
[architecture.md](architecture.md); this page is the reference.

## TypeScript

Import from `@facile/antenne`. Seven client exports, plus the envelope and catalog contract
described under [Envelope and catalog (TypeScript)](#envelope-and-catalog-typescript).

### Types

```ts
export interface AntenneConfig {
  app: string;
  instance: string;
  secret: string;
  instance_id?: string;
  events?: { emit?: string[]; listen?: string[] };
}

export interface AntenneManifest {
  app: string;
  instance?: string;
  secret?: string;
  instance_id?: string;
  events?: { emit?: string[]; listen?: string[] };
}

export interface AntenneClientOptions {
  config?: AntenneConfig;
  configPath?: string;
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Error) => void;
  reconnect?: boolean;              // default true
  maxReconnectAttempts?: number;    // default 20
}

export interface EventMeta {
  id: string;
  channel: string;
  offset: number;
  timestamp: number;   // epoch milliseconds
  sender: string;      // "app" or "app:instance_id"
}
```

`EventHandler<T>` and `Unsubscribe` are declared but **not exported**, so a consumer that wants
to name a handler type has to write it out: `(payload: T, meta: EventMeta) => void | Promise<void>`.
Note also that a handler may return a promise, but nothing awaits it — a rejected promise from
an async handler is unhandled, not routed to `onError`.

### Functions

| Function | Signature | Behavior |
|---|---|---|
| `loadManifest` | `loadManifest(path?: string): AntenneManifest` | Reads the YAML and validates only that `app` is a non-empty string |
| `loadConfig` | `loadConfig(path?: string): AntenneConfig` | `loadManifest`, then requires `instance` and `secret` |

Both throw when no path is given and no `antenne.yaml` / `antenne.yml` is found in the working
directory and `ANTENNE_CONFIG_PATH` does not resolve.

### `AntenneClient`

`new AntenneClient(options: AntenneClientOptions)` — with no `options.config`, the constructor calls
`loadConfig(options.configPath)` and therefore **throws synchronously** if the file is missing
or incomplete.

| Member | Signature | Notes |
|---|---|---|
| `connect` | `connect(): Promise<void>` | Registers, then opens the socket. Rejects on a registration failure or a socket error raised before the connection is live |
| `disconnect` | `disconnect(): Promise<void>` | Clears the reconnect flag, cancels a pending retry, closes with code 1000 |
| `emit` | `emit<T>(channel: string, payload: T): void` | Sends, or queues in memory when disconnected. Never reports failure |
| `emitNow` | `emitNow<T>(channel: string, payload: T): boolean` | Sends only when the socket is `OPEN`; returns `false` instead of queuing |
| `listen` | `listen<T>(channel: string, handler): () => void` | Returns an unsubscribe function that removes exactly that handler |
| `isConnected` | `get isConnected(): boolean` | |
| `currentEpoch` | `get currentEpoch(): string` | The `epoch` from the last `welcome`; empty before the first one |
| `identity` | `identity(): string` | `app`, or `app:instance_id` |

Handlers registered on the same channel run in insertion order. A synchronous throw is caught
and routed to `onError`; the remaining handlers still run, and the event is still acked.

### Envelope and catalog (TypeScript)

`envelope.ts` and `catalog.ts` mirror `go/envelope` and `go/catalog`: types and constants only.

| Export | Kind | Go counterpart |
|---|---|---|
| `FacileApp`, `FacileAction`, `FacileObjectType` | string unions | `envelope.App`, `Action`, `ObjectType` |
| `FacileEvent<T>` | interface | `envelope.Event[T]` |
| `FACILE_EVENT_VERSION` | `1` | `envelope.EventVersion` |
| `FacileProject`, `FacileTask`, `FacileUser`, `FacileTimeEntry`, `FacileAgentSession`, `FacileInvoice`, `FacileDocument`, `FacileMonitor` | interfaces | the payload struct of the same name |
| `Channels` | `as const` object | the `catalog` constants, same names |
| `Channel` | string union of every `Channels` value | `catalog.Channel` |
| `allChannels()` | `Channel[]`, declaration order | `catalog.All()` |

`ts/test/parity.test.ts` builds one typed value per fixture in `go/envelope/testdata/` and
asserts `JSON.stringify` equals the fixture, then checks `allChannels()` against the constants
parsed out of `go/catalog/catalog.go`. `bun run test` typechecks the test first, so a renamed
key fails at compile time on the TypeScript side and at the byte comparison on the Go side.

## Go

Import as `pool "github.com/FacileStudio/antenne-client/go"`. Sixteen top-level exports in the
root package, plus the three contract packages below.

### Types

```go
type Config struct {
	App        string      `yaml:"app" json:"app"`
	Instance   string      `yaml:"instance" json:"instance"`
	Secret     string      `yaml:"secret" json:"secret"`
	InstanceID string      `yaml:"instance_id" json:"instance_id"`
	Events     EventConfig `yaml:"events" json:"events"`
}

type EventConfig struct {
	Emit   []string `yaml:"emit" json:"emit"`
	Listen []string `yaml:"listen" json:"listen"`
}

type EventMeta struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Offset    int64  `json:"offset"`
	Timestamp int64  `json:"timestamp"` // epoch milliseconds
	Sender    string `json:"sender"`
}

type EventHandler func(payload json.RawMessage, meta EventMeta)
```

The payload reaches a Go handler as `json.RawMessage` — re-marshaled from the decoded message,
so field order is not preserved. Unmarshal it into your own type.

`var ErrNotConnected = errors.New("pool: not connected")` is the sentinel returned by `EmitNow`.
Compare it with `errors.Is`.

### Constructors and options

| Symbol | Signature |
|---|---|
| `NewClient` | `NewClient(config *Config, opts ...ClientOption) *Client` |
| `NewClientFromYAML` | `NewClientFromYAML(path string, opts ...ClientOption) (*Client, error)` |
| `LoadConfig` | `LoadConfig(path string) (*Config, error)` — pass `""` to auto-detect |
| `ClientOption` | `func(*Client)` |
| `WithOnConnect` | `WithOnConnect(fn func()) ClientOption` |
| `WithOnDisconnect` | `WithOnDisconnect(fn func()) ClientOption` |
| `WithOnError` | `WithOnError(fn func(error)) ClientOption` |
| `WithMaxReconnect` | `WithMaxReconnect(n int) ClientOption` — default 20 |

`LoadConfig` requires `app`, `instance` and `secret` together; there is no manifest-only mode.

| Symbol | Signature |
|---|---|
| `ConfigFromEnv` | `ConfigFromEnv(app string) (*Config, error)` |
| `ErrNotConfigured` | sentinel returned by `ConfigFromEnv` when no URL is set |

`ConfigFromEnv` reads the instance URL from `ANTENNE_URL`, or from `ANTENNE_INSTANCE` when
`ANTENNE_URL` is empty, and trims a trailing `/`. `ANTENNE_APP` overrides the `app` argument.
`ANTENNE_SECRET` and `ANTENNE_INSTANCE_ID` fill their fields. A URL without a secret or an app
name is an error, distinct from `ErrNotConfigured`. `Events` is left empty for the caller.

### `Client`

| Method | Signature | Notes |
|---|---|---|
| `Connect` | `Connect(ctx context.Context) error` | Registers, then dials. Errors are wrapped as `registration failed:` or `websocket connect failed:` |
| `Disconnect` | `Disconnect()` | Clears the reconnect flag, sends a close frame, closes the connection. No error returned |
| `Emit` | `Emit(channel string, payload any) error` | Sends, or appends to an in-memory queue. Only a marshal or write error is returned — a queued send is `nil` |
| `EmitNow` | `EmitNow(channel string, payload any) error` | Returns `ErrNotConnected` rather than queuing |
| `Listen` | `Listen(channel string, handler EventHandler) func()` | See below |
| `IsConnected` | `IsConnected() bool` | |
| `Identity` | `Identity() string` | `App`, or `App:InstanceID` |

The `ctx` passed to `Connect` bounds registration and the dial only. It does not cancel the
client: the read, ping and reconnect loops run on background goroutines with their own
contexts, and a reconnect uses a fresh 30 s context of its own. `Disconnect` is the only way to
stop them.

There is no exported accessor for the connection epoch on the Go side, and no equivalent of
TypeScript's `reconnect: false`.

#### `Listen`

The returned function is **currently a no-op**. It compares `&handlers[i] == &handler` — the
address of a slice element against the address of the parameter variable — which can never be
equal, so the handler is never removed. Do not rely on unsubscribing in Go; keep handlers for
the lifetime of the client, or gate them yourself. TypeScript's `listen` unsubscribes correctly.

Registering a handler after `Connect` also does not subscribe to its channel on the current
connection — only on the next reconnect. Register handlers before connecting.

## Go: `envelope`

Import `github.com/FacileStudio/antenne-client/go/envelope`. The content of
`github.com/FacileStudio/enveloppe/go`, with identical JSON keys.

| Symbol | What it is |
|---|---|
| `App` | string type; constants `AppSablier`, `AppOpus`, `AppArdoise`, `AppPlume`, `AppGlouton`, `AppVision`, `AppMycelium`, `AppSonde`, `AppAgenda`, `AppJournal`, `AppCourrier`, `AppNuage`, `AppBulle`, `AppAntenne` |
| `Action` | string type; `ActionCreated`, `ActionUpdated`, `ActionDeleted` |
| `ObjectType` | string type; `ObjectProject`, `ObjectTask`, `ObjectUser`, `ObjectTimeEntry`, `ObjectInvoice`, `ObjectDocument`, `ObjectAgentSession`, `ObjectMonitor` |
| `EventVersion` | `1`. Additive changes do not bump it |
| `Event[T]` | `version`, `app`, `object`, `action`, `facile_id`, `payload`, `timestamp`, `idempotency_key` |
| `Project`, `Task`, `User`, `TimeEntry`, `AgentSession`, `Invoice`, `Document`, `Monitor` | payload structs |

Changing a payload means changing the struct, the TypeScript interface and the fixture in
`go/envelope/testdata/` together. Both test suites fail until all three agree.

## Go: `catalog`

Import `github.com/FacileStudio/antenne-client/go/catalog`.

| Symbol | What it is |
|---|---|
| `Channel` | string type |
| 25 constants | one per channel, each documented with its emitter and payload type |
| `All() []Channel` | every constant, in declaration order |

The test parses `catalog.go` and fails when a constant is missing from `All()`, appears twice,
or does not match `^[a-z][a-z_]*(\.[a-z_]+)+$`.

| Channels | Emitter | Payload |
|---|---|---|
| `task.created`, `task.updated`, `task.deleted` | Opus, Sablier | `Event[Task]` |
| `task.status_changed`, `task.actor_changed` | Opus | `Event[Task]` |
| `project.created`, `project.updated`, `project.deleted` | Opus, Sablier | `Event[Project]` |
| `time_entry.created`, `time_entry.updated`, `time_entry.deleted` | Sablier | `Event[TimeEntry]` |
| `comment.created`, `workspace.created` | Opus | `Event` of an Opus-local payload |
| `agent_session.created` | Mycelium | `Event[AgentSession]` |
| `agent_session.updated` | none today; Sablier listens | `Event[AgentSession]` |
| `usage_alert.created` | Mycelium | `Event` of a Mycelium-local payload |
| `alert.fired` | Journal | `Event` of a map |
| `monitor.down`, `monitor.up` | Sonde | `Event[Monitor]` |
| `git.push`, `git.pull_request`, `git.issue`, `git.event` | Antenne git webhook | Antenne's normalized forge payload |
| `filet.check.failed`, `filet.check.passed` | Antenne filet webhook | Antenne's normalized filet run |

## Go: `outbox`

Import `github.com/FacileStudio/antenne-client/go/outbox`. Depends on `database/sql` only.

| Symbol | Signature | Notes |
|---|---|---|
| `Migration` | `string` | goose SQL, Up and Down, creating `antenne_outbox`. Copy it into your migrations |
| `Enqueue` | `Enqueue(ctx, tx *sql.Tx, channel catalog.Channel, payload any, idempotencyKey string) error` | Call inside the business transaction |
| `ErrDuplicate` | sentinel | The key is already queued. Uses `ON CONFLICT DO NOTHING`, so `tx` stays usable |
| `ErrEmptyKey` | sentinel | An empty idempotency key is refused |
| `NewDrainer` | `NewDrainer(db *sql.DB, emit func(channel string, payload any) error, opts ...Option) *Drainer` | Pass `client.EmitNow`, never `Emit` |
| `(*Drainer).Run` | `Run(ctx context.Context)` | Drain and prune every interval until `ctx` is done |
| `(*Drainer).Drain` | `Drain(ctx) (int, error)` | One pass; returns rows sent |
| `(*Drainer).Prune` | `Prune(ctx) (int64, error)` | Deletes rows sent longer ago than the retention |
| `Option` | `func(*Drainer)` | |
| `WithInterval` | default 2 s | Pause between passes |
| `WithBatchSize` | default 100 | Rows locked per pass |
| `WithRetention` | default 7 days | Age at which sent rows are pruned |
| `WithMaxBackoff` | default 1 min | Cap on the doubling pause after failed passes |
| `WithOnError` | `func(error)` | Receives every failed pass |

The table:

```sql
antenne_outbox(
  id bigserial primary key,
  channel text not null,
  payload jsonb not null,
  idempotency_key text not null unique,
  created_at timestamptz not null default now(),
  sent_at timestamptz
)
```

One pass opens a transaction, selects unsent rows `ORDER BY id LIMIT n FOR UPDATE SKIP
LOCKED`, emits each one as `json.RawMessage`, and sets `sent_at` row by row. The first emit
failure stops the pass and commits what was sent, so that row and every later one stay unsent.
A second drainer skips the locked rows and takes the next ones, so no row is sent twice, but
across drainers the order is only per batch.

## Behavioral parity

Same protocol, two implementations, and a handful of gaps that are real rather than accidental.
The table is maintained in [development.md](development.md#keeping-the-two-halves-aligned).
