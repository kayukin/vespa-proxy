# ─────────────────────────────────────────────────────────────────────────────
# Stage 1 – Build the Vespa UI (Node / Yarn)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS ui-builder

RUN apk add --no-cache git

WORKDIR /src

ARG VESPA_VERSION=master
RUN git clone \
      --depth 1 \
      --filter=blob:none \
      --sparse \
      https://github.com/vespa-engine/vespa.git \
      /src/vespa
WORKDIR /src/vespa
RUN git sparse-checkout set client/js/app

WORKDIR /src/vespa/client/js/app

RUN yarn install --frozen-lockfile
RUN yarn build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 1b – Export-only target: copy the built UI to the host with
#
#   docker build --target ui-export --output type=local,dest=./ui-dist .
#
# Skipped entirely during a normal `docker build`.
# ─────────────────────────────────────────────────────────────────────────────
FROM scratch AS ui-export
COPY --from=ui-builder /src/vespa/client/js/app/dist/ /

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2 – Build the Go proxy binary
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS go-builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY main.go ./
COPY internal/ ./internal/

# Embed the compiled UI
COPY --from=ui-builder /src/vespa/client/js/app/dist/ ./internal/ui/static/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /vespa-proxy \
      .

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3 – Minimal runtime image
# ─────────────────────────────────────────────────────────────────────────────
FROM scratch

COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=go-builder /vespa-proxy /vespa-proxy

# ── Configuration ────────────────────────────────────────────────────────────
#
#  Mount your config.yaml and pass its path via the -config flag:
#
#    docker run \
#      -v $(pwd)/config.yaml:/config.yaml:ro \
#      -v $(pwd)/certs:/certs:ro \
#      vespa-proxy -config /config.yaml
#
#  The config file supports ${ENV_VAR} substitution, so secrets can be
#  injected at runtime without modifying the file itself.
#
EXPOSE 8080

ENTRYPOINT ["/vespa-proxy"]
