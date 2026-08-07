# pool — API

The complete exported surface of both implementations. Protocol behavior lives in
[architecture.md](architecture.md); this page is the reference.

## TypeScript

Import from `@facile/antenne`. Seven exports.

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

## Go

Import as `pool "github.com/FacileStudio/antenne-client/go"`. Fourteen top-level exports.

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

## Behavioral parity

Same protocol, two implementations, and a handful of gaps that are real rather than accidental.
The table is maintained in [development.md](development.md#keeping-the-two-halves-aligned).
