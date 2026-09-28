# http-proxy

A small reverse proxy that forwards **every** incoming HTTP request to `TARGET`, preserving
method, headers, body and query, with WebSocket upgrades and streamed responses passed
straight through. A single Go executable that installs itself as a native Windows service
(no NSSM, no Node.js).

See [docs/architecture.md](docs/architecture.md) for how it works and
[docs/deployment.md](docs/deployment.md) for installing it on the server; after that, every
push to `main` is built by GitHub Actions and deployed by ElvareConsole (section 5 of that file).

## Build

Requires Go 1.26+. From Git Bash:

```bash
./build.sh              # writes httpproxy.exe
```

## Configure

Copy `.env.example` to `.env` next to the exe and set at least `TARGET`. The keys of the
old Node.js version (`PORT`, `TARGET`, `SECURE`, `PROXY_TIMEOUT_MS`, `LOGGING`) mean the
same thing, so an existing `.env` carries over.

## Run

```bash
httpproxy.exe serve                    # foreground (default command)
httpproxy.exe service install          # then: service start | stop | restart | uninstall
                                       # --name / --display-name / --description, or SERVICE_* in .env
```

`GET /healthz` is answered by the proxy itself (`HEALTH_PATH`, `off` to forward it too).
As a service it logs to daily files `httpproxy-YYYY-MM-DD.log` in `LOG_DIR` (default `logs\`
next to the exe), keeping `LOG_KEEP_DAYS` days (default 30). `LOGGING=true` adds one line per
request.

## Develop

```bash
go run ./cmd/httpproxy serve           # needs TARGET (env or .env)
go test ./...
```
