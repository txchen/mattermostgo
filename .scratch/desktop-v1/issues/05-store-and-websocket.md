# Store, reducer and WebSocket

Type: task
Status: resolved
Blocked by: 01, 03

`internal/store` model and reducer with generated-sequence tests;
`internal/mm` WebSocket with auth, backoff reconnect, wake handling and
refetch after reconnect. Spec feature 8.

## Comments

2026-11-12: internal/store with generated-sequence invariant tests (TestGeneratedSequences, 500 seeds x 200 events); internal/mm socket with Bearer auth, backoff 1 s to 5 min with jitter, wake on resume and network change, refetch after reconnect, a 100 s silence watchdog.
