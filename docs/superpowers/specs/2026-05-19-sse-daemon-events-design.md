# SSE Daemon Events — Design Spec

**Date:** 2026-05-19
**Status:** Approved
**Scope:** Daemon heartbeat pulse animation + indeterminate scan progress bar

---

## Problem

The daemon status bar is a static green/red dot polled every 10 seconds. There is no way to know when a scan is running or when rules evaluation has finished without waiting for the next poll cycle. Two UX gaps:

1. No visual feedback that the daemon is actively processing emails.
2. No signal to the user that a scan just completed (heartbeat).

---

## Solution Overview

Add a Server-Sent Events (SSE) endpoint to the backend. The daemon coordinator emits three typed events into a fan-out broker. The frontend opens one persistent `EventSource` connection and uses the events to drive two UI behaviors:

- **Indeterminate progress bar** inline in the status bar while a scan is running.
- **Ping/ripple animation** on the green dot when rules evaluation completes (heartbeat).

---

## Backend

### `internal/events` package

New package `backend/internal/events` with a single exported type:

```go
type EventType string

const (
    EventScanStarted  EventType = "scan_started"
    EventHeartbeat    EventType = "heartbeat"
    EventScanError    EventType = "scan_error"
)

type Event struct {
    Type                EventType `json:"type"`
    Reader              string    `json:"reader"`
    Timestamp           time.Time `json:"timestamp"`
    TransactionsWritten int       `json:"transactions_written,omitempty"` // heartbeat only
    Error               string    `json:"error,omitempty"`                // scan_error only
}

type Broker struct { /* sync.Map of subscriber channels */ }

func NewBroker() *Broker
func (b *Broker) Subscribe() (<-chan Event, func())  // returns channel + unsubscribe fn
func (b *Broker) Emit(e Event)                       // non-blocking; drops slow clients
```

`Emit` is non-blocking: each subscriber channel is buffered (size 8). If the buffer is full, the subscriber is dropped from the map and its channel is closed — this prevents a slow or disconnected client from blocking the daemon goroutine.

### Event emission points (in `cmd/server/main.go`)

| Event | Where emitted |
|-------|--------------|
| `scan_started` | After `dm.setRunning(time.Now())` in `runDaemon` |
| `heartbeat` | After `runner.Run()` returns `nil` error in `runDaemon` |
| `scan_error` | After `runner.Run()` returns a non-cancellation error in `runDaemon` |

`daemonCoordinator` receives a `*events.Broker` at construction. `runDaemon` receives it as a parameter.

### SSE endpoint

**Route:** `GET /api/events`

Registered in `registerRoutes`. Handler:

1. Sets headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no` (disables nginx buffering).
2. Calls `broker.Subscribe()` to get a channel and defer the unsubscribe cleanup.
3. Sends a `: keepalive` comment every 25 seconds to prevent proxy/load-balancer timeouts.
4. On each received event, writes:
   ```
   event: <type>\n
   data: <json>\n
   \n
   ```
   and flushes via `http.Flusher`.
5. Returns when the request context is cancelled (client disconnects).

No authentication is added to this endpoint — it follows the same pattern as all other `/api/*` routes (CORS middleware handles cross-origin; the app is single-user/self-hosted).

The server's `WriteTimeout` must be set to 0 (or a very large value) for SSE connections. The existing 30-second write timeout in `server.go` will prematurely terminate SSE streams. This will be changed to use a custom handler that bypasses the timeout, or the server timeout will be increased to match the keepalive interval.

---

## Frontend

### `src/hooks/useSSE.ts`

```ts
type SSEEvent = {
  type: 'scan_started' | 'heartbeat' | 'scan_error'
  reader: string
  timestamp: string
  transactions_written?: number
  error?: string
}

function useSSE(onEvent: (e: SSEEvent) => void): void
```

- Opens `new EventSource('/api/events')` on mount.
- Listens for named events (`scan_started`, `heartbeat`, `scan_error`) via `addEventListener`.
- Parses `event.data` as JSON and calls `onEvent`.
- On `onerror`: `EventSource` natively reconnects with exponential backoff — no manual retry logic needed.
- Closes and cleans up on unmount.

### `DaemonStatusBar` changes

Two new pieces of state:

```ts
const [scanState, setScanState] = useState<'idle' | 'scanning'>('idle')
const [pingActive, setPingActive] = useState(false)
```

Event handlers (passed to `useSSE`):

| Event | Action |
|-------|--------|
| `scan_started` | `setScanState('scanning')` |
| `heartbeat` | `setScanState('idle')`, `setPingActive(true)`, schedule `setPingActive(false)` after 1400ms |
| `scan_error` | `setScanState('idle')` |

**Render logic:**

- `scanState === 'scanning'`: replace the "daemon running · uptime X" row with an indeterminate blue progress bar (full-width, CSS `@keyframes` sweep).
- `scanState === 'idle'` + `pingActive === true`: render the green dot with a `::after` pseudo-element doing the ping/ripple animation (expanding ring, fades out, `animation-iteration-count: 1`).
- `scanState === 'idle'` + `pingActive === false`: existing static dot + text.
- Daemon stopped/error: existing red dot behavior, no ping, no progress bar regardless of SSE state.

### CSS for indeterminate bar

```css
@keyframes indeterminate {
  0%   { transform: translateX(-100%); }
  100% { transform: translateX(200%); }
}
```

A `overflow-hidden` wrapper with a child `div` that is 50% wide and runs the animation on `linear infinite`. Blue (`#3b82f6`) to match the "scanning" label color.

### CSS for ping/ripple

```css
@keyframes ping {
  0%   { transform: scale(1); opacity: 0.8; }
  100% { transform: scale(2.8); opacity: 0; }
}
```

Applied via `animate-ping` Tailwind class or an inline `@keyframes` on the `::after` pseudo-element of the green dot. `animation-iteration-count: 1`.

---

## WriteTimeout consideration

`server.go` sets `WriteTimeout: 30 * time.Second`. SSE connections are long-lived — the 30s timeout will kill them. Fix: set `WriteTimeout: 0` on the HTTP server (no write timeout). The 25s keepalive comment ensures proxies don't consider the connection idle.

---

## Error Handling

- If the SSE connection drops (network error, server restart), `EventSource` reconnects automatically. The frontend will re-subscribe and receive future events.
- If the browser tab is hidden for a long time and the connection times out, reconnection happens on next user interaction (standard `EventSource` behavior).
- `scan_error` is emitted but the dot color is already driven by the 10s poll of `/api/status` — the SSE event just ensures the progress bar is cleared immediately rather than waiting for the next poll.

---

## What This Does Not Cover

- Per-email progress counts (indeterminate bar is used instead).
- Scan progress for retroscan vs initial scan (both use the same `scan_started` / `heartbeat` events — the UI treats them identically).
- Authentication on the SSE endpoint (not needed; app is self-hosted single-user).
