# The Go half: cmd/api and cmd/migrate.
#
# Both binaries ship in one image because they must be built from the same
# commit. A migrate image that lags the API by a deploy applies the wrong
# schema, and one that leads it runs migrations the running code has never
# seen -- the two are a single unit of deployment, so they are a single image
# and compose picks which entrypoint to run.

# ── build ───────────────────────────────────────────────────────────────────
# Pinned to the version go.mod asks for. A pin older than the directive does
# not fail: GOTOOLCHAIN quietly downloads the newer toolchain mid-build
# instead, which turns a hermetic build into a network dependency.
FROM golang:1.26.5-alpine AS build

WORKDIR /src

# Dependencies first, as their own layer. They change on the rare commit that
# touches go.mod; source changes on nearly every one, and putting them in the
# same layer would re-download the module cache for a one-line edit.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

# CGO off makes a static binary, which is what lets the runtime stage below be
# alpine with nothing in it rather than a full toolchain.
#
# -trimpath keeps the build machine's directory layout out of panic traces.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ── runtime ─────────────────────────────────────────────────────────────────
FROM alpine:3.22 AS runtime

# ca-certificates: the process makes outbound TLS calls of its own -- SMTP on
#   587, and the eSewa/Khalti status endpoints a payment is confirmed against.
#   Without them every gateway verification fails certificate validation and
#   no online booking can ever be confirmed.
# tzdata: ARENA_TIMEZONE goes through time.LoadLocation, which reads the
#   system zone database. Without it config.Load fails at boot with
#   "Asia/Kathmandu is not a known zone" -- caught at startup rather than in
#   production, which is the point of loading it eagerly.
RUN apk add --no-cache ca-certificates tzdata

# An uploads directory the API can write to, owned by the user that runs it.
# Created here rather than left to the volume mount so the ownership is right
# even on a fresh volume, which Docker creates as root.
RUN addgroup -S khel && adduser -S -G khel khel && \
    mkdir -p /var/lib/khel/media && chown -R khel:khel /var/lib/khel

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/migrate /usr/local/bin/migrate

USER khel

ENV HTTP_ADDR=:8080 \
    MEDIA_DIR=/var/lib/khel/media

EXPOSE 8080

# Liveness only. /healthz deliberately checks no dependency: this probe
# restarts the container when it fails, and restarting the API does not fix a
# database that is down. Compose reads /readyz for that -- see compose.yaml.
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/api"]
