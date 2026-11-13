# Contributing

Mattermost Go is a Go program built with [mygo](https://github.com/egoist/mygo).
This page covers building it, testing it against a local server, and the
scripts that check the real window. [RELEASING.md](RELEASING.md) covers
signing and publishing.

## Build and run

You need Go 1.27 on macOS 12 or later.

```sh
go run ./cmd/mattermostgo        # run from source
go tool mygo dev                 # rebuild and restart on every edit
go tool mygo build               # build/darwin-arm64/Mattermost Go.app and a dmg
```

## Test

```sh
go test ./...                    # unit and view tests, no server needed
scripts/devserver.sh start       # a local Mattermost server on :8065, seeded
MM_E2E_URL=http://localhost:8065 MM_E2E_MFA=/tmp/mmserver/mfa-secret go test ./internal/e2e
scripts/devserver.sh stop
```

`scripts/devserver.sh` needs `brew install postgresql@17` and builds the
server from source, so it needs no Docker. The seeded users are `alice`,
`bob`, `carol` and `mfauser`, all with the password `Password1!`.
Set `MM_FRAMES=<dir>` to save what the window shows during the view and
end-to-end tests as PNGs. `MMGO_DEBUG=1` logs the WebSocket messages.

Development builds link mygo's automation plugin, which listens only
when `MYGO_AUTOMATION` names a socket; release builds leave it out.
Two scripts drive the real window that way, against the dev server:

```sh
go install github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
go tool mygo build -debug
open -n "build/darwin-arm64/Mattermost Go.app" --env MYGO_AUTOMATION=/tmp/mmapp.sock \
  --env MYGO_FRAME_STATS=all --stderr /tmp/mmapp.log
# log in as alice in the window, then:
MM_APP="build/darwin-arm64/Mattermost Go.app" scripts/realwindow.sh  # 18 interactions, checked on the server
go run ./scripts/seed -big 5000 && scripts/perf/bigscroll.sh         # frame times scrolling 5,000 posts
```

The window must be on screen for frame times: macOS does not present a
window while the display sleeps. `go test -bench ScrollBigChannel
./internal/view` measures the same scroll without a window.

## Screenshots

`go run ./scripts/screenshot -out docs/screenshots` draws the window
headless with made-up data and writes the pictures of the README, in the
light and the dark theme.

## Measurements

[docs/benchmarks.md](docs/benchmarks.md) has the comparison with the
official client and how to measure it again. `scripts/perf/compare.sh`
compares two builds of this app.

## Documents

- [Analysis](docs/analysis.md) explains what the official client does and what mygo offers.
- [Design](docs/design.md) covers the architecture, data model and performance rules.
- [ADRs](docs/adr/) record the decisions and why we made them.
- [v1 spec](.scratch/desktop-v1/spec.md) defines the scope, and the [issues](.scratch/desktop-v1/issues/) are the work items.
- [Glossary](GLOSSARY.md) defines the domain vocabulary.
