# Mattermost Go

A native desktop client for [Mattermost](https://mattermost.com), written in Go
with [mygo](https://github.com/egoist/mygo). It draws its whole interface with
mygo's native UI toolkit, with no webview and no bundled browser engine, and
talks to the Mattermost server through its REST API and WebSocket.

Goals, in order:

1. Small and frugal. A single binary of a few megabytes, near-zero CPU when
   idle, memory well under the official Electron client.
2. The everyday chat features first. Rare or advanced features are tracked
   in [the deferred list](.scratch/desktop-v1/spec.md#deferred) instead of built.
3. macOS first. Linux and Windows build but are not verified yet.

Status: design. Nothing is implemented yet.

## Documents

- [Analysis](docs/analysis.md) explains what the official client does and what mygo offers.
- [Design](docs/design.md) covers the architecture, data model and performance rules.
- [ADRs](docs/adr/) record the decisions and why we made them.
- [v1 spec](.scratch/desktop-v1/spec.md) defines the scope, and the [issues](.scratch/desktop-v1/issues/) are the work items.
- [Glossary](GLOSSARY.md) defines the domain vocabulary.

## License

Apache 2.0. Mattermost is a trademark of Mattermost, Inc. This project is
not affiliated with Mattermost, Inc.
