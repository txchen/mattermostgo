# Verify the Mattermost API and WebSocket contract

Type: research
Status: resolved

Confirm every endpoint, field and WebSocket event listed as unverified in
docs/analysis.md against the official API reference and a running server.
Record JSON samples as fixtures under `internal/mm/testdata/`. Find out
whether the server offers an SSO flow usable by a third-party native client
(browser login ending in a deep link with a token), and note it in the
spec's deferred list.

## Comments

2026-11-12: Checked against the server source at 9e3a32bc and a server built from it. Corrections are in docs/analysis.md (multiple_channels_viewed, Bearer header on the WebSocket, channel_members?page=-1, MFA error IDs). Fixtures are written from the model types in internal/mm/fake_test.go rather than recorded; the end-to-end tests in internal/e2e run the client against the real server instead. SSO stays deferred: not investigated further.
