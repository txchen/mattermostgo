# Servers and login

Type: task
Status: resolved
Blocked by: 01, 03

Spec features 1 and 2. Server list in `internal/config`, tokens in the
Keychain, `Session` sum type, login with MFA, logout, expired token handling.

## Comments

2026-11-12: internal/config (servers.json), Keychain through mygo.Secrets read and written off the main thread, app.Session sum type, MFA login verified end to end (TestMFALogin), expired sessions return to the login form.
