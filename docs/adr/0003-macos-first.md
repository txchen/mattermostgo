# 0003. macOS first, built and verified on a Mac

Status: accepted, 2026-11-12

## Decision

v1 targets macOS 12 or later (mygo's default minimum), on Apple Silicon and Intel as a universal
binary. Development and verification happen on a Mac. Linux and Windows
must keep compiling (mygo is pure Go), but nobody checks their behavior
until a later version.

The app is called Mattermost Go, with its own bundle ID
`com.github.txchen.mattermostgo` (to change if the project moves to an
organization) so it installs next to the official client. It does not read
the official client's configuration.

Signing and notarization use a Developer ID the maintainer provides at
release time.
