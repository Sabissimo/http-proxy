# Architecture

`httpproxy.exe` is a transparent reverse proxy: every request that reaches `PORT` is sent to
`TARGET` and the answer is streamed back. It replaces the earlier Node.js version
(`proxy-http-proxy.js` on `http-proxy`, run under NSSM) and keeps its configuration keys.

## Layout

| Path | Role |
|---|---|
| `cmd/httpproxy` | entry point |
| `internal/cli` | Cobra commands: `serve` (default), `service <action>`; sets up logging before any command |
| `internal/config` | `.env` + environment via Viper: proxy settings, service registration, log folder |
| `internal/server` | the listener: local health check, everything else to the proxy; graceful shutdown |
| `internal/proxy` | `httputil.ReverseProxy` to `TARGET`, 502 on upstream failure, optional request log |
| `internal/logging` | slog to the console, or to daily files when running as a service |
| `internal/winsvc` | Windows service install/control via `kardianos/service` |

## Request path

1. `server.Handler` answers `GET`/`HEAD` on `HEALTH_PATH` (default `/healthz`) with `200 ok`
   without touching the upstream. Any other method on that path, and every other path, goes on.
2. `proxy.New` rewrites the request onto `TARGET`:
   - the target's path is prefixed (`TARGET=https://h/app`: `/x?q=1` → `https://h/app/x?q=1`);
   - `Host` becomes the target's host (http-proxy's `changeOrigin`);
   - `X-Forwarded-For` / `-Host` / `-Proto` are set, appending to an incoming
     `X-Forwarded-For` (`xfwd`);
   - hop-by-hop headers are dropped, as HTTP requires.
3. The response is copied back and flushed chunk by chunk (`FlushInterval: -1`), so
   server-sent events, long polling and downloads are not held back.
4. A `Connection: Upgrade` request (WebSocket) is tunnelled: once the upstream answers `101`,
   both connections are joined byte for byte until either side closes.

**Why not Fiber:** fasthttp buffers bodies and cannot hand over a raw connection for an
upgrade, so the stdlib `ReverseProxy` is used here — the one deviation from the usual stack.

## Upstream TLS and timeouts

- `SECURE=false` (default, as before) accepts any upstream certificate; `true` verifies it.
- `PROXY_TIMEOUT_MS` (default 60000, `0` = no limit) bounds the wait for the upstream's
  response **headers** only. A response that has started, or an open WebSocket, runs as long
  as it needs.
- Upstream unreachable, refused, or too slow → `502 Bad gateway` (plain text), logged as a
  warning. A client that disconnects mid-request is not logged as an error.

## Logging

`internal/logging` (shared with ElvareDriveBackup) sets the default slog logger before any
command runs:

- **console** (`serve` from a terminal): text to stderr;
- **service**: `httpproxy-YYYY-MM-DD.log` in `LOG_DIR` (default `logs\` next to the exe; a
  relative path is resolved against the exe's folder, since a service starts in System32).
  The writer switches file at local midnight and deletes files older than `LOG_KEEP_DAYS`
  (default 30, `0` keeps all); only files matching that name are ever deleted.

Always logged: startup (address, target, settings), upstream errors, shutdown. With
`LOGGING=true`, also one line per request once it finishes — method, URL, status, duration,
client address; a WebSocket's line is written when it closes, its duration being the
connection's lifetime.

## Service

`service install` registers the exe to start as `httpproxy.exe service run --name <name>`,
auto-start, LocalSystem. `run` is what the service manager calls: it starts the server with
a context the manager cancels on stop, after which in-flight requests get 10 s to finish.
Name, display name and description come from flags, else `SERVICE_*` in `.env`, else
`HttpProxy` / `HTTP Proxy` / a default description.

## Build and deploy

`build.sh` cross-compiles `httpproxy.exe` (`CGO_ENABLED=0`, windows/amd64).
`.github/workflows/build.yml` runs `go vet`, `go test`, the build, and uploads the exe as the
artifact `httpproxy` on every push to `main`; ElvareConsole deploys it (see
[deployment.md](deployment.md)).
