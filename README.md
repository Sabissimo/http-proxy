# Node Reverse Proxy (port 7777)

A tiny Node.js reverse proxy that forwards **all** incoming HTTP requests to `TARGET`, preserving method, headers, body, and query. Supports WebSockets and streaming.

## Quick start

```bash
npm install
# copy and edit env
cp .env.example .env
# (on Windows you can copy manually)
# Edit .env and set TARGET to your upstream URL

npm start
# or
node proxy-http-proxy.js
