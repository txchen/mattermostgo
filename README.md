<div align="center">

<img src="resources/icon.png" width="96" alt="">

# Mattermost Go

**A native Mattermost client for macOS, with builds for Linux and Windows. No Electron, no browser engine.**

[![Release](https://img.shields.io/github/v/release/txchen/mattermostgo)](https://github.com/txchen/mattermostgo/releases/latest)
![macOS 12+](https://img.shields.io/badge/macOS-12%2B-blue)
![Linux and Windows](https://img.shields.io/badge/Linux%20%7C%20Windows-preview-orange)
![Apple Silicon and Intel](https://img.shields.io/badge/Apple%20Silicon%20%7C%20Intel-universal-lightgrey)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)

</div>

![Mattermost Go, with a channel and a thread open](docs/screenshots/light.png)

Mattermost Go talks to your Mattermost server through its API and draws
the whole interface natively in Go. It works like the official app (the
same sidebar, threads, search and integrations) at a fraction of its
size and memory, and next to no CPU while it waits in the background.

## Smaller and lighter

Measured on an Apple M1 against the official Mattermost Desktop 6.3.2:

| | Mattermost Go | Official desktop app |
|---|---:|---:|
| Download | **9 MB** | 232 MB |
| App on disk | **30 MB** | 296 MB |
| Processes | **1** | 11 |
| Memory | **98 MB**, logged in with a channel open | 338 MB, before adding a server |
| CPU in the background, per minute | **under 0.01 s** | 1.0 s |

Both download sizes are of the universal apps. The official app was
measured on its first screen, before it loads the webapp, so its real use
is higher. [How these were measured](docs/benchmarks.md), and how to
check them on your own server.

## Install

### macOS

With [Homebrew](https://brew.sh):

```sh
brew install --cask txchen/tap/mattermost-go
```

Or download the dmg from the [latest release](https://github.com/txchen/mattermostgo/releases/latest).
The app is signed and notarized, runs on macOS 12 or later, and is one
universal app for Apple Silicon and Intel Macs. It installs next to the
official app and does not touch its settings.

The app updates itself: it checks for a new version once a day, shows
what is new, and installs it when you agree ("Check for Updates…" in the
account menu). Homebrew leaves it to, so `brew upgrade` does not touch it.

### Linux and Windows (preview)

Builds for x86-64 and ARM64 come from the same code as the Mac app, but
nobody has tested them on these systems yet: please
[report](https://github.com/txchen/mattermostgo/issues) what you find.

**Linux**, for your user, without root, and updating itself:

```sh
curl -fsSL https://github.com/txchen/mattermostgo/releases/latest/download/install.sh | sh
```

Or install the `.deb` (Debian, Ubuntu) or `.rpm` (Fedora, openSUSE) from the
[latest release](https://github.com/txchen/mattermostgo/releases/latest)
with `sudo apt install ./mattermost-go_VERSION_amd64.deb` or
`sudo dnf install ./mattermost-go-VERSION-1.x86_64.rpm`. These do not update
themselves: install the next release's package over them.

**Windows 10 and 11**, in PowerShell, for your user, and updating itself:

```powershell
irm https://github.com/txchen/mattermostgo/releases/latest/download/install.ps1 | iex
```

Or run `mattermost-go-setup-VERSION-amd64.exe` from the latest release. The
installer is not signed yet, so SmartScreen asks before it runs: choose
*More info*, then *Run anyway*.

There too the app checks for a new version once a day and installs it
when you agree. Updates are signed, and the app checks the signature
before it installs one.

The Linux packages need GTK 3, which desktop distributions have.

## What it does

**Chat**
- Channels, direct and group messages, and Collapsed Reply Threads, with the
  Threads view of the threads you follow.
- Markdown with code highlighting, tables, emoji, mentions and channel links;
  link previews and message attachments.
- Reactions, edits, pinned and saved messages, reminders, forwarding, and
  edit history.
- An emoji picker with skin tones and your server's custom emoji.
- Files by drag, paste or picker, and a preview for images, text and code,
  with Quick Look for the rest.
- Real-time posts, reactions, typing and read state across your devices.

**Find things**
- Search messages and files, Recent mentions, Saved messages.
- Jump to any post from a result or a link.
- Find channels and people with ⌘K, and go back and forward with ⌘[ and ⌘].

**Organize**
- Sidebar categories you can drag, sort, mute and collapse; favorites;
  unread channels grouped first if you like.
- Create, browse, rename, archive and unarchive channels; edit headers and
  purposes; manage members and channel admins; convert between public and
  private.
- Open any channel or thread in a window of its own.

**Integrations**
- Incoming and outgoing webhooks, slash commands and bot accounts with
  tokens, created and edited in the app.
- Buttons, menus and dialogs from bots and integrations.
- Slash commands complete as you type.

**You**
- Status, Do Not Disturb with an end time, and custom status.
- Your profile, picture, password, multi-factor authentication, sessions
  and personal access tokens.
- Desktop notifications with your server's rules, per-channel preferences,
  and the Dock badge.
- Light and dark themes, including soft dark ones (One Dark, Nord, Dusk)
  that follow the system if you want; 12 or 24 hour clock; compact
  messages.
- Several servers at once, each with its own login.

![The Threads view and the theme settings, in One Dark](docs/screenshots/dark.png)

## Not there yet

- Single sign-on (GitLab, Google, Office 365, SAML, OpenID). Logging in
  takes a username or email and a password, with MFA.
- Plugins with their own interface, such as Calls, Boards and Playbooks.
- Servers with self-signed certificates, client certificates, and proxies
  that ask for a password.
- Uploading custom emoji, and push and email notification settings.
- Tested Linux and Windows builds. The packages are there, but nobody has
  tried them yet, and the Windows installer is not signed.

## Privacy

The app talks only to the servers you add. Session tokens stay in the
system keychain (the macOS Keychain, the Secret Service on Linux, the
Credential Manager on Windows). Link previews come from your server, which fetches them;
their images load straight from the sites they are on. There is no
telemetry.

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) covers building from source, the test
server, and the end-to-end and real-window checks. The
[design](docs/design.md) and the [decisions](docs/adr/) explain how the
app is put together.

## License

Apache 2.0. Mattermost is a trademark of Mattermost, Inc. This project is
not affiliated with or endorsed by Mattermost, Inc.
