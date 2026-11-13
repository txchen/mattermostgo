# Scaffold the app and a local test server

Type: task
Status: resolved

`go.mod`, `cmd/mattermostgo` opening one empty native window with the app
name, bundle ID and placeholder icon, `mygo` packaging config, and a
`docker compose` file plus a seed script that starts a Mattermost server
with two teams, users, channels and posts for manual and end-to-end testing.

## Comments

2026-11-12: go.mod, cmd/mattermostgo and mygo.json. No Docker on this Mac, so scripts/devserver.sh builds the server from source and runs it on Homebrew PostgreSQL 17; scripts/seed fills it. The icon is still the mygo default.
