// Load .env if present
require('dotenv').config();

const fs = require('fs');
const path = require('path');
const http = require('http');
const httpProxy = require('http-proxy');

// ---- Config ----
const PORT = parseInt(process.env.PORT || '7777', 10);
const TARGET = process.env.TARGET || 'https://new.example.com';
const SECURE = String(process.env.SECURE || 'false').toLowerCase() === 'true';
const PROXY_TIMEOUT_MS = parseInt(process.env.PROXY_TIMEOUT_MS || '60000', 10);
const LOGGING = String(process.env.LOGGING || 'false').toLowerCase() === 'true';

// ---- Optional logging setup ----
let log = () => {};
if (LOGGING) {
  const LOG_DIR = path.join(__dirname, 'logs');
  fs.mkdirSync(LOG_DIR, { recursive: true });
  const LOG_FILE = path.join(LOG_DIR, 'proxy.log');
  log = (...args) => {
    const line = `[${new Date().toISOString()}] ${args.join(' ')}\n`;
    try { fs.appendFileSync(LOG_FILE, line); } catch {}
    console.log(...args);
  };
} else {
  // minimal fallback to still show critical errors
  log = (...args) => {};
  console.log(`Logging disabled. To enable, set LOGGING=true`);
}

process.on('uncaughtException', (err) => LOGGING && console.error('Uncaught:', err));
process.on('unhandledRejection', (err) => LOGGING && console.error('UnhandledRejection:', err));

// ---- Proxy setup ----
const proxy = httpProxy.createProxyServer({
  target: TARGET,
  changeOrigin: true,
  xfwd: true,
  secure: SECURE,
  proxyTimeout: PROXY_TIMEOUT_MS
});

proxy.on('error', (err, req, res) => {
  if (LOGGING) console.error('Proxy error:', err?.message);
  if (!res.headersSent) {
    res.writeHead(502, { 'Content-Type': 'text/plain' });
  }
  res.end('Bad gateway');
});

// ---- HTTP Server ----
const server = http.createServer((req, res) => {
  if (LOGGING) log('>>>', req.method, req.url);
  proxy.web(req, res);
});

server.on('listening', () => {
  console.log(`Reverse proxy listening on ${PORT} -> ${TARGET} (secure=${SECURE})`);
});
server.on('error', (e) => LOGGING && console.error('Server error:', e?.message));
server.listen(PORT);

// Keep alive
setInterval(() => {}, 1 << 30);

// WebSockets
server.on('upgrade', (req, socket, head) => {
  if (LOGGING) log('WS upgrade:', req.url);
  proxy.ws(req, socket, head);
});
