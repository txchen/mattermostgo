# Verify the Mattermost API and WebSocket contract

Type: research
Status: open

Confirm every endpoint, field and WebSocket event listed as unverified in
docs/analysis.md against the official API reference and a running server.
Record JSON samples as fixtures under `internal/mm/testdata/`. Find out
whether the server offers an SSO flow usable by a third-party native client
(browser login ending in a deep link with a token), and note it in the
spec's deferred list.
