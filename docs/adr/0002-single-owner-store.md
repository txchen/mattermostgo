# 0002. One store per server, changed only by a reducer on the main thread

Status: accepted, 2026-11-12

## Context

Data arrives from many goroutines: HTTP responses, the WebSocket reader,
image downloads. mygo's view reads state on the main thread, and
`Window.Update` runs a function there and draws a frame.

## Decision

Network code never touches UI state. It parses what it receives into typed
events at the boundary (`internal/mm`) and hands them to the main thread
with `Window.Update`. There, one reducer applies each event to the store.
The store has no locks because only the main thread reads or writes it.

## Consequences

- No data races by construction, and no mutexes to reason about.
- Every state change is one function call with a typed input, so the store
  is tested without a window or a network.
- Slow work (JSON parsing, image decoding) must finish off the main thread
  before the event is sent, or frames stall.
