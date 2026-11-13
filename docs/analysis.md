# Analysis

Written 2026-11-12 from shallow clones of `mattermost/desktop` (version
6.5.0-develop.1, Electron 45.2.1) and `egoist/mygo` (Go 1.27.3). Statements
marked *unverified* come from memory of the Mattermost API and must be
checked against the API reference before code relies on them (issue 01).

## What the official desktop client is

The official client is a shell around the Mattermost webapp. Electron's main
process (`src/main`) manages servers, windows, downloads, notifications,
certificates, auto-update and the tray. The renderer (`src/renderer/components`)
draws only the frame around the chat: `TopBar`, `ServerDropdownButton`,
`NewServerModal`, `SettingsModal`, `DownloadsDropdown`, `WelcomeScreen`,
error and loading views. The chat itself is the webapp's React UI, loaded
from each server into a web view.

So the official client's cost has two parts. Electron ships Chromium and
Node in the package. Every server also runs a full webapp in its own
renderer process.

## Why a webview shell is not enough

A webview shell on mygo would drop the bundled Chromium, because WKWebView
ships with macOS. It would keep the second cost. The webapp is a large React
application, and each server would still run it in a WebKit content process.
The goal is to save resources, so we rewrite the chat UI natively too
([ADR 0001](adr/0001-native-ui-no-webview.md)).

What we give up by not running the webapp:

- Server plugins with webapp UI do not show (Boards, Playbooks, Calls, and
  third-party plugins).
- Every chat feature is ours to build and keep up with server changes.
- Interactive message attachments, dialogs and slash command UIs need native
  equivalents, one by one.

This is why v1 picks the everyday features and defers the rest.

## What mygo gives us

From `docs/ui/README.md` and the other guides under `docs/ui`:

- A GPU-drawn native UI (Metal on macOS) written in Go, with a view function
  of the state. A frame is drawn when the state changes or the view asks for
  one (`Window.Update`, `Window.Invalidate`, `c.Invalidate`, `c.After`), so a
  still window costs no frames.
- `ui.List` builds only the rows in view, with variable row heights,
  `FollowEnd` for chat, and a loading-more hook. Its benchmark shows the
  visible rows of a million-row list at 14.5 µs a frame (`docs/ui/performance.md`,
  measured by mygo on Apple M5, not by us).
- `ui/markdown`, a CommonMark 0.31.2 and GFM parser with a chat example
  (`examples/markdown-chat`), selection and copy, links and images, and code blocks.
- Rich text editor and text input with input methods, which matters for CJK input.
- Images (bitmaps, SVG), popovers, menus, dialogs, tooltips, toasts, sidebar, split views.
- Desktop APIs: notifications, `App.SetBadgeCount` and `App.Dock.SetBadge`,
  Keychain secrets (`secrets.go`), deep links, single instance, tray, global
  shortcuts, window state, signed dmg packaging and notarization.
- Pure Go with no cgo, so the build needs only Go.

Open questions about mygo, settled by the prototype in issue 02:

- Can `ui/markdown` take Mattermost's inline extensions (`@user`,
  `~channel`, `:emoji:`)? `New(ext)` takes extensions, but whether user
  extensions are possible is unknown.
- Memory and frame cost of a channel with a few thousand posts, images and
  emoji, on a real Mac.
- Animated GIF support. Selection across several posts.

## The Mattermost API surface v1 needs

Checked in issue 01 against the server source (`mattermost/mattermost`
at 9e3a32bc, `server/channels/api4` and `server/channels/app`) and a
running server built from it. Corrections to what this section first
said are marked **Corrected**.

- Login: `POST /api/v4/users/login` with `login_id`, `password` and the
  optional MFA `token`. The response header `Token` is the session token.
- Me, teams, channels: `GET /api/v4/users/me`, `/users/me/teams`,
  `/users/me/teams/{team_id}/channels`, `/users/me/channels` for DMs and GMs.
- Read state: `GET /users/me/channel_members?page=-1` streams every
  channel member of the user, one JSON object a line, so one request
  covers all teams, DMs and GMs. `POST /api/v4/channels/members/me/view`
  with `channel_id` and `collapsed_threads_supported` marks a channel read.
- An MFA account without a code fails login with the error ID
  `mfa.validate_token.authenticate.app_error`; a wrong code with
  `api.user.check_user_mfa.bad_code.app_error`. The server takes each
  TOTP code once.
- Sidebar categories: `/users/me/teams/{team_id}/channels/categories`.
- Posts: `GET /channels/{id}/posts?page&per_page&before`,
  `POST /posts`, `PUT /posts/{id}/patch`, `DELETE /posts/{id}`,
  `GET /posts/{id}/thread`.
- Reactions: `POST /reactions`, `DELETE /users/me/posts/{id}/reactions/{name}`.
- Files: `POST /files`, `GET /files/{id}`, `/files/{id}/thumbnail`, `/files/{id}/preview`.
- Users: `POST /users/ids`, `GET /users/{id}/image`, `POST /users/status/ids`.
- WebSocket `/api/v4/websocket`. **Corrected:** the handshake takes the
  session in an `Authorization: Bearer` header, so no
  `authentication_challenge` is needed; `hello` confirms it. The server
  pings every 60 s and drops a connection silent for 100 s. Events v1
  handles: `posted` (with `mentions`, a JSON array of user IDs, written
  as a string), `post_edited`, `post_deleted`, `reaction_added`,
  `reaction_removed`, `typing`, `status_change`, `user_added`,
  `user_removed`, `user_updated`, `channel_created`, `channel_updated`,
  `channel_deleted`, `channel_member_updated`, `direct_added`,
  `group_added`, `sidebar_category_*`, `added_to_team`, `leave_team`,
  and `hello`. **Corrected:** a channel read elsewhere arrives as
  `multiple_channels_viewed` with `channel_times`, not `channel_viewed`.
  Data fields holding objects (`post`, `reaction`, `channel`,
  `channelMember`) are JSON written as strings, except `user`.
  The client sends `user_typing`. After a reconnect the client refetches
  the open channel and the unread counts, since missed events are not replayed.
- SSO (GitLab, Google, Office 365, SAML, OpenID). The official client opens
  the server's login page in a window. A native client needs a browser
  round trip that ends in a deep link with a token. Whether the server
  offers such a flow for third-party clients is the open question that
  blocks SSO, so SSO is deferred.

## UI to copy

The look follows the Mattermost webapp, since that is what users see in the
official client: a server picker in the top bar as in `TopBar` and
`ServerDropdownButton`, a team strip on the left, the channel sidebar with
categories, the channel header, the post list, the composer at the bottom,
and the RHS for threads. Colors come from the webapp's default themes
(Denim light, Onyx dark). The logo is derived from the official app icon in
`src/assets` (Apache 2.0), changed so that it is distinguishable.
