# Deployment

`httpproxy.exe` runs as a Windows service on the machine that should accept the traffic.

## 1. `.env`

Put `httpproxy.exe` in its install folder (e.g. `D:\Services\HttpProxy`), copy
`.env.example` to `.env` next to it and set `TARGET` (and `PORT` if not 7777). The comments
in `.env.example` cover the rest.

## 2. Install the service

From an elevated prompt in the install folder:

```powershell
.\httpproxy.exe serve                   # optional: try it in the foreground, Ctrl+C to stop
.\httpproxy.exe service install
.\httpproxy.exe service start
```

The name, display name and description can be set with flags or in `.env`
(`SERVICE_NAME`, `SERVICE_DISPLAY_NAME`, `SERVICE_DESCRIPTION`); flags win, and the defaults
are `HttpProxy` / `HTTP Proxy`. Nothing is added to the name you give:

```powershell
.\httpproxy.exe service install --name ElvareProxy --display-name "Elvare Proxy"
```

start / stop / restart / uninstall need the same name — pass the same `--name`, or keep it in
`.env` so every command picks it up. The installed service runs as LocalSystem, auto-start,
and logs to `httpproxy-YYYY-MM-DD.log` in `LOG_DIR` (default `logs\` next to the exe).

Check it: `curl http://localhost:<PORT>/healthz` → `ok`.

## 3. Allow the port

If other machines connect to it:

```powershell
New-NetFirewallRule -DisplayName "HTTP Proxy" -Direction Inbound -Protocol TCP -LocalPort 7777 -Action Allow
```

## 4. Moving from the Node.js version

The old version ran `node proxy-http-proxy.js` under NSSM. The `.env` keys are the same, so:

1. Copy its `.env` next to `httpproxy.exe` (the NSSM service's folder has it).
2. Stop and remove the old service — both cannot listen on the same port:
   `nssm stop <old name>` then `nssm remove <old name> confirm`.
3. Install and start the new one (section 2). To keep the old service name, pass it as
   `--name` after removing the old one.

Differences: startup and upstream errors are now always logged (to daily files as a
service); `LOGGING=true` only adds the per-request lines. `GET /healthz` is answered by the
proxy — set `HEALTH_PATH=off` if the upstream's own `/healthz` must be reachable through it,
or move the proxy's elsewhere (e.g. `HEALTH_PATH=/_proxy/healthz`).

## 5. Continuous deployment

GitHub Actions (`.github/workflows/build.yml`) builds every push to `main` (Markdown and
`docs/` excepted): `go vet`, `go test` and the exe on a GitHub Windows runner, uploaded as the
artifact `httpproxy`. **ElvareConsole** (https://devops.elvare.ge) deploys it once the app is
registered there (*Settings → Apps → Add app*):

| Name | Repository | Workflow | Branch | Artifact | Executable | Services |
|---|---|---|---|---|---|---|
| proxy | `Sabissimo/http-proxy` | `build.yml` | `main` | `httpproxy` | `httpproxy.exe` | the service name from section 2 |

The console stops the service, swaps `httpproxy.exe`, starts it, checks
`http://localhost:<PORT>/healthz`, and puts the previous exe back if the new one does not come
up — so keep the proxy's health check at `/healthz` on a console-deployed instance.
*Actions → build → Run workflow* rebuilds `main` without a push.

`.env` is never touched by a deploy — a new key goes into the server's `.env` (the console has
an editor for it) before the push. A deploy restarts the service, dropping open connections
(WebSockets included) for a moment.
