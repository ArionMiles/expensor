# Multi-stage Dockerfile for Expensor
# Stage 1 builds the React frontend; stage 2 builds the Go binary;
# stage 3 produces a minimal runtime image with both.

# ─── Stage 1: Frontend ───────────────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder

WORKDIR /build/frontend

# Install dependencies with frozen lockfile for reproducible builds
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Build the production bundle (output: frontend/dist/)
COPY frontend/ .
RUN npm run build

# ─── Stage 2: Backend ────────────────────────────────────────────────────────
FROM golang:1.26.1-alpine AS backend-builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify

COPY backend/ .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}" \
    -o expensor ./cmd/server

RUN test -x ./expensor && test -s ./expensor

# ─── Stage 3: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

RUN addgroup -g 1000 expensor && \
    adduser -D -u 1000 -G expensor expensor

WORKDIR /app

# Copy the Go binary
COPY --from=backend-builder /build/backend/expensor /app/expensor

# Copy the built frontend assets — served by the binary at runtime
COPY --from=frontend-builder /build/frontend/dist /app/public

# Create data directory for credentials, tokens, and state file
RUN mkdir -p /app/data && chown -R expensor:expensor /app

USER expensor

EXPOSE 8080

# Tell the binary where to find the frontend assets
ENV EXPENSOR_STATIC_DIR=/app/public

# Volume for persistent data (mount ./data:/app/data)
VOLUME ["/app/data"]

ENTRYPOINT ["/app/expensor"]

LABEL org.opencontainers.image.title="Expensor"
LABEL org.opencontainers.image.description="Expense tracker that reads Gmail/Thunderbird and writes to PostgreSQL"
LABEL org.opencontainers.image.url="https://github.com/ArionMiles/expensor"
LABEL org.opencontainers.image.source="https://github.com/ArionMiles/expensor"
LABEL org.opencontainers.image.vendor="ArionMiles"
LABEL org.opencontainers.image.licenses="MIT"
