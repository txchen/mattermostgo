# Mattermost Go v1

Goal: a native macOS Mattermost client for daily chat on a recent server,
light on CPU, memory and disk. Design in [docs/design.md](../../docs/design.md).

## Target

- macOS 12 or later, universal binary. Linux and Windows compile only.
- The latest Mattermost server release. No compatibility work for old servers.
- English UI only.
- Email or username and password login, with MFA.

## v1 features

Ordered by priority. Each item lists what "done" means.

1. **Servers.** Add (URL and display name, URL checked with
   `/api/v4/system/ping`), edit, remove, switch from the top bar dropdown.
   Remembered across restarts. Tokens in the Keychain.
2. **Login and logout.** Password and MFA. An expired token returns the
   server to the login form without losing other servers.
3. **Teams and sidebar.** Team strip, channel sidebar with the server's
   categories, unread bold, mention count, muted channels dimmed, DMs with
   status dots.
4. **Channel view.** Header (name, purpose, member count), post list
   with infinite scroll back, date separators, consecutive posts from one
   user grouped, Markdown (CommonMark, GFM tables, code blocks,
   `@mentions` highlighted, `~channel` links, emoji), edited marker,
   image thumbnails and file cards, reactions shown.
5. **Composer.** Multiline input, Enter sends, Shift+Enter breaks a line,
   Up edits the last own post, `@` user autocomplete, `~` channel
   autocomplete, `:` emoji autocomplete, attach files by picker, paste and
   drag.
6. **Post actions.** Reply, edit, delete own post, add or remove a
   reaction from a small recent set, copy text, copy link.
7. **Threads.** Open a thread in the RHS, read and reply.
8. **Real time.** New, edited and deleted posts, reactions, typing,
   statuses, channel read on another device, channels added or removed.
   Reconnects on its own and shows connection state.
9. **Read state.** Opening a channel marks it read. A "new messages" line
   at the first unread post.
10. **Notifications and badge.** macOS notifications per the user's
    settings, click opens the channel. Dock badge with mentions across servers.
11. **Navigation.** Cmd+K channel and DM switcher, open a DM with any
    user, Cmd+[ and Cmd+] for history, Option+Up/Down for channels.
12. **Links.** Web links open in the default browser. Permalinks to the
    same server open in the app. `mattermost://` deep links open the server
    and channel.
13. **Basics.** Light and dark theme following the system, window state
    remembered, single instance, About window, a quit that leaves no process.
14. **Packaging.** Signed and notarized dmg named Mattermost Go, own icon
    derived from the official one, own bundle ID.

Acceptance numbers, measured on the same Mac against the official client:
idle CPU near 0% over 60 s with one server, RSS at most half of the
official client's with three servers, dmg under 20 MB.

## Deferred

Tracked here instead of built. Move an item into v1 only with a reason.

Sign-in and network:

- SSO: GitLab, Google, Office 365, SAML, OpenID. Out of v1 by decision
  (2026-11-12). Needs a browser login
  flow that ends in a deep link (see docs/analysis.md).
- Untrusted server certificates (self-signed or private CA): asking the
  user to trust one and remembering it.
- Client certificates (mutual TLS) chosen from the Keychain.
- Authenticated HTTP proxies asking for a username and password.

Product:

- Message search, saved and pinned posts, recent mentions.
- Collapsed Reply Threads "Threads" view and following threads.
- Channel management: create, rename, archive, members, convert.
- Full emoji picker, custom emoji upload, skin tones.
- Link previews (OpenGraph) and embedded YouTube.
- Interactive message attachments, buttons and menus, interactive dialogs,
  slash command autocomplete. Plain slash commands are sent as text.
- Scheduled posts, drafts synced across devices, post priority and acknowledgements.
- User profile and account settings editing, notification settings UI
  (v1 reads the server's settings but does not edit them).
- Custom themes from the user's Mattermost preferences.
- Calls, Boards, Playbooks and any plugin UI.
- Downloads manager window (v1 saves to Downloads and reveals in Finder).
- Spell check, if mygo text input does not provide it.
- LaTeX, inline images in Markdown from external hosts beyond a size cap.
- Pop-out windows for channels or threads.

Platform:

- Auto-update.
- Launch at login, tray or menu bar icon.
- Managed or GPO configuration.
- Languages other than English.
- Linux and Windows verification and packages.
