# pool — Configuration

Every key the client reads, in both languages, and the two environment-dependent lookups.
Configuration is either passed inline or loaded from a YAML file; there is no merging of the
two.

## Keys

| Key | Type | Required | What it does |
|---|---|---|---|
| `app` | string | yes | App name. Becomes the pool identity and the `sender` on emitted events |
| `instance` | string | yes | Antenne base URL. The WebSocket URL replaces the first `http` with `ws` |
| `secret` | string | yes | Shared secret, sent in the registration body |
| `instance_id` | string | no | Makes the identity `app:instance_id` |
| `events.emit` | string[] | no | Channels the app emits on. **Never read by the client** |
| `events.listen` | string[] | no | Channels subscribed on connect |

Both `events.emit` and `events.listen` are forwarded verbatim in the registration body, so Antenne
sees the full declaration. Only `events.listen` has a client-side effect: it seeds the
`subscribe` message. `events.emit` is documentation for Antenne and for humans — nothing in the
client validates an emit against it, so emitting on an undeclared channel is not a client-side
error.

`instance` must carry a scheme. The WebSocket URL is built by replacing the **first** `http`
occurrence with `ws`, so `http://` becomes `ws://` and `https://` becomes `wss://`. A schemeless
value produces a URL the dialer rejects.

## `antenne.yaml`

Placed at the project root. Both `antenne.yaml` and `antenne.yml` are accepted, in that order,
resolved relative to the **current working directory** — not the module or package directory.

```yaml
app: opus
instance: http://localhost:3400
secret: shared-secret
events:
  emit:
    - project.created
    - project.updated
  listen:
    - project.created
    - project.updated
```

If neither file is found in the working directory, `ANTENNE_CONFIG_PATH` is consulted and used
when it points at a file that exists.

| Variable | Required | Default | What it does |
|---|---|---|---|
| `ANTENNE_CONFIG_PATH` | no | — | Fallback path to the YAML file, used only when no `antenne.yaml`/`antenne.yml` is in the working directory |

## Environment (Go, `ConfigFromEnv`)

`ConfigFromEnv(app)` builds a `Config` without a YAML file. The suite spells the URL two ways,
so both are read.

| Variable | Required | Default | What it does |
|---|---|---|---|
| `ANTENNE_URL` | no | — | Antenne base URL. Unset together with `ANTENNE_INSTANCE` returns `ErrNotConfigured`, meaning publishing is off |
| `ANTENNE_INSTANCE` | no | — | Synonym for `ANTENNE_URL`, read only when `ANTENNE_URL` is empty |
| `ANTENNE_SECRET` | when a URL is set | — | Shared secret sent at registration |
| `ANTENNE_APP` | no | the `app` argument | Overrides the app name |
| `ANTENNE_INSTANCE_ID` | no | — | Makes the identity `app:instance_id` |

## Full config versus manifest (TypeScript)

TypeScript splits the shape in two, because in practice apps commit a `antenne.yaml` that
documents channels while injecting connection details from the environment:

- **`AntenneConfig`** — `app`, `instance`, `secret` all required. What `AntenneClient` needs.
- **`AntenneManifest`** — only `app` required; `instance`, `secret` and `instance_id` optional.

`loadManifest(path?)` parses the file and validates only that `app` is a non-empty string.
`loadConfig(path?)` calls it and then requires `instance` and `secret`, throwing
`antenne.yaml: 'instance' and 'secret' are required for full config` otherwise. Reading a
channel-only `antenne.yaml` therefore needs `loadManifest`, not `loadConfig`.

Go has no equivalent split. `LoadConfig(path)` requires `app`, `instance` and `secret`
together, and fails with `config: app, instance, and secret are required` if any is missing —
so a Go app cannot load a channel-only manifest through this library.

This is what the three consumers in the suite actually do: their `antenne.yaml` carries `app` and
`events` only, and the API builds the full config inline in code. Sablier's file says so in a
comment, and so does Opus's:

```yaml
# Documents the pool events Sablier uses. The API builds its pool config
# inline (modules/antenne/service.go) — keep this file in sync with it.
app: Sablier
events:
  emit:
    - time_entry.created
```

Nothing enforces that the file and the code agree. Treat drift between them as expected and
check the code when they disagree.

## Client options

Behavior knobs live on the client, not in the YAML.

| TypeScript (`AntenneClientOptions`) | Go (`ClientOption`) | Default | What it does |
|---|---|---|---|
| `config` | first argument to `NewClient` | — | Inline config; when absent, TS falls back to `loadConfig(configPath)` |
| `configPath` | `NewClientFromYAML(path, …)` | — | Explicit YAML path |
| `reconnect` | — | `true` | TS only. `false` disables automatic reconnection |
| `maxReconnectAttempts` | `WithMaxReconnect(n)` | `20` | Attempts before the client gives up for good |
| `onConnect` | `WithOnConnect(fn)` | — | Fired after each successful open, including reconnects |
| `onDisconnect` | `WithOnDisconnect(fn)` | — | Fired on a close that follows a live connection |
| `onError` | `WithOnError(fn)` | — | Socket errors, server `error` messages, and giving up on reconnect |

Go has no equivalent of `reconnect: false`; reconnection is always on and can only be stopped
by calling `Disconnect`. In TypeScript, `onError` also receives anything a handler throws, so a
throwing listener is reported rather than swallowed. In Go a panicking handler is not
recovered and will take the read loop down with it.

## Secrets

The registration `secret` and the returned token are held in memory only — nothing is written
to disk, and neither is logged by this library. The token does travel in the WebSocket URL's
query string, so it reaches any proxy access log in the path. Every Facile API redacts `token`
from its own request logs for exactly this reason; the same care is worth taking at the edge.
