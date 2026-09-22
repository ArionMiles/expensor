# Multi-stage Dockerfile for Expensor
# Stage 1 builds the React frontend (architecture-independent, built once).
# Stage 2 builds the Go binary for the target platform.
# Stage 3 produces the minimal runtime image.

# ─── Stage 1: Frontend ───────────────────────────────────────────────────────
# --platform=$BUILDPLATFORM runs this stage on the build host (amd64) regardless
# of the target platform. The frontend bundle is architecture-independent, so
# there is no need to run npm under QEMU.
FROM --platform=$BUILDPLATFORM node:26-alpine AS frontend-builder

WORKDIR /build/frontend

# Install dependencies with frozen lockfile for reproducible builds
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Copy frontend source, including frontend-owned static content.
COPY frontend/ .
RUN npm run build

# ─── Stage 2: Backend ────────────────────────────────────────────────────────
# --platform=$BUILDPLATFORM keeps the Go toolchain running natively on the build
# host. Cross-compilation is handled purely via GOOS/GOARCH env vars below —
# no QEMU required even when targeting linux/arm64.
FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine AS backend-builder

# Install build tooling needed by the Go toolchain.
# This layer is cached as long as the base image doesn't change.
# ARGs that vary per build (VERSION, TARGETOS, TARGETARCH) are declared BELOW
# this step so they don't invalidate this layer's cache on every release.
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build/backend

COPY backend/go.mod backend/go.sum ./

# --mount=type=cache persists the module cache across builds on the same
# BuildKit daemon (warm local builds, persistent self-hosted runners).
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    go mod download && go mod verify

COPY backend/ .

# Stage the frontend where the production build embeds it.
COPY --from=frontend-builder /build/frontend/dist ./internal/httpapi/dist

# ARGs are declared here so that changing VERSION/platform does NOT invalidate
# the apk, go mod download, or COPY layers above.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -tags production -trimpath -ldflags="-s -w -X github.com/ArionMiles/expensor/backend/pkg/config.Version=${VERSION}" \
    -o expensor ./cmd/server

RUN test -x ./expensor && test -s ./expensor

# ─── Release artifacts ───────────────────────────────────────────────────────
FROM backend-builder AS release-builder

ARG VERSION=dev

COPY --from=frontend-builder /build/frontend/dist /build/frontend/dist
COPY scripts/release /build/scripts/release
COPY deploy/config.toml.example /build/deploy/config.toml.example
COPY LICENSE NOTICE /build/

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    /build/scripts/release/package.sh "${VERSION}" /build/dist/release

FROM scratch AS release-artifacts

COPY --from=release-builder /build/dist/release /

# ─── Stage 3: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

RUN addgroup -g 1000 expensor && \
    adduser -D -u 1000 -G expensor expensor

WORKDIR /app

# Copy the Go binary
COPY --from=backend-builder /build/backend/expensor /app/expensor

# Prepare a private SQLite directory that is copied into new named volumes.
RUN mkdir -p /app/data/expensor && \
    chmod 0700 /app/data/expensor && \
    chown -R expensor:expensor /app

USER expensor

EXPOSE 8080

ENTRYPOINT ["/app/expensor"]

# Static OCI annotations — fallback for local builds.
# CI workflows override these with dynamic values (created, revision, version)
# via docker/metadata-action and also write them to the manifest index.
LABEL org.opencontainers.image.title="Expensor" \
      org.opencontainers.image.description="Email-driven personal finance tracker with SQLite and PostgreSQL storage" \
      org.opencontainers.image.url="https://github.com/ArionMiles/expensor" \
      org.opencontainers.image.source="https://github.com/ArionMiles/expensor" \
      org.opencontainers.image.documentation="https://github.com/ArionMiles/expensor#readme" \
      org.opencontainers.image.vendor="ArionMiles" \
      org.opencontainers.image.licenses="AGPL-3.0"
