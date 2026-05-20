# vespa-proxy

A Go reverse-proxy that:

1. **Embeds** the Vespa UI ([`client/js/app`](https://github.com/vespa-engine/vespa/tree/master/client/js/app)) as a compiled static SPA.
2. **Proxies** all API requests to a Vespa Cloud endpoint.
3. **Adds mTLS** client-certificate authentication to every upstream request.

The entire build is self-contained in a multi-stage Dockerfile — no local Node.js or Go installation required.

---

## Architecture

```
Browser
  │
  ▼
[ vespa-proxy :8080 ]
  │
  ├── GET /              → embedded Vespa UI (SPA)
  ├── GET /assets/*      → embedded static assets
  │
  ├── /api/*             → strip /api prefix, proxy to VESPA_URL (mTLS)
  ├── /search/*          → proxy to VESPA_URL (mTLS)
  └── /document/*        → proxy to VESPA_URL (mTLS)
```

---

## Quick Start

### 1. Obtain mTLS credentials from Vespa Cloud

### 2. Configure

```bash
cp example.config.yaml config.yaml
# edit config.yaml
```

### 3. Build & run with Docker Compose

```bash
mkdir -p certs
# place client.pem, client.key (and optionally ca.pem) in ./certs/

docker compose up --build
```

The proxy will be available at <http://localhost:8080>.

---

## Configuration Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `VESPA_URL` | ✅ | — | Vespa Cloud base URL, e.g. `https://my-app.vespa-app.cloud` |
| `LISTEN_ADDR` | | `:8080` | HTTP server bind address |
| `UPSTREAM_TIMEOUT_SEC` | | `30` | Timeout for upstream requests (seconds) |
| **mTLS — files** | | | |
| `TLS_CERT_FILE` | ✅* | — | Path to client certificate PEM file |
| `TLS_KEY_FILE` | ✅* | — | Path to client private key PEM file |
| `TLS_CA_FILE` | | — | Path to CA bundle PEM file (server verification) |
| **mTLS — inline PEM** | | | Overrides file paths when set |
| `TLS_CERT_PEM` | ✅* | — | Inline client certificate PEM |
| `TLS_KEY_PEM` | ✅* | — | Inline client private key PEM |
| `TLS_CA_PEM` | | — | Inline CA bundle PEM |
| `TLS_SKIP_VERIFY` | | `false` | Skip TLS server verification (**dev only**) |

\* At least one cert+key pair (file or inline) is required for mTLS.

---

## Project Structure

```
.
├── Dockerfile                  # 3-stage build: Node UI → Go binary → scratch
├── docker-compose.yml
├── go.mod
├── main.go                     # HTTP server, SPA handler, logging middleware
├── internal/
│   ├── config/
│   │   └── config.go           # Environment-based configuration
│   ├── proxy/
│   │   ├── proxy.go            # Reverse proxy with mTLS transport
│   │   └── context.go          # Request timeout helper
│   └── ui/
│       ├── ui.go               # //go:embed directive
│       └── static/             # Populated by Dockerfile at build time
└── README.md
```

---

## Updating the UI version

```bash
# Pin to a specific Vespa release tag
docker build --build-arg VESPA_VERSION=8.400.0 -t vespa-proxy:8.400.0 .
```

---

## Security Notes

- **Never** commit private key files. Use Docker secrets or a secrets manager in production.
- `TLS_SKIP_VERIFY=true` disables server certificate verification — only for local development.
- The final Docker image is based on `scratch` (zero OS layer), minimising the attack surface.
