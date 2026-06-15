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

# Copy frontend source; then materialise the symlink target so Vite can
# resolve frontend/public/readers -> ../../content/readers at build time.
COPY frontend/ .
COPY content/readers/ /build/content/readers/
RUN npm run build

# ─── Stage 2: Backend ────────────────────────────────────────────────────────
# --platform=$BUILDPLATFORM keeps the Go toolchain running natively on the build
# host. Cross-compilation is handled purely via GOOS/GOARCH env vars below —
# no QEMU required even when targeting linux/arm64.
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS backend-builder

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

# ARGs are declared here so that changing VERSION/platform does NOT invalidate
# the apk, go mod download, or COPY layers above.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w -X github.com/ArionMiles/expensor/backend/pkg/config.Version=${VERSION}" \
    -o expensor ./cmd/server

RUN test -x ./expensor && test -s ./expensor

# ─── Stage 3: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

RUN addgroup -g 1000 expensor && \
    adduser -D -u 1000 -G expensor expensor

WORKDIR /app

# Copy the Go binary
COPY --from=backend-builder /build/backend/expensor /app/expensor

# Copy the built frontend assets — served by the binary at runtime
COPY --from=frontend-builder /build/frontend/dist /app/public

# Create the legacy import directory. Current runtime state is stored in PostgreSQL,
# but upgraded installs can still mount /app/data for one-time file import.
RUN mkdir -p /app/data && chown -R expensor:expensor /app

USER expensor

EXPOSE 8080

# Tell the binary where to find the frontend assets
ENV EXPENSOR_STATIC_DIR=/app/public

ENTRYPOINT ["/app/expensor"]

# Static OCI annotations — fallback for local builds.
# CI workflows override these with dynamic values (created, revision, version)
# via docker/metadata-action and also write them to the manifest index.
LABEL org.opencontainers.image.title="Expensor" \
      org.opencontainers.image.description="Expense tracker that reads Gmail/Thunderbird and writes to PostgreSQL" \
      org.opencontainers.image.url="https://github.com/ArionMiles/expensor" \
      org.opencontainers.image.source="https://github.com/ArionMiles/expensor" \
      org.opencontainers.image.documentation="https://github.com/ArionMiles/expensor#readme" \
      org.opencontainers.image.vendor="ArionMiles" \
      org.opencontainers.image.licenses="AGPL-3.0"
