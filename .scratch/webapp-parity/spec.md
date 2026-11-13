# Webapp parity, round 1

Status: done (see decisions.tsv); a click on the text of a post selects it rather than opening the thread

The user compared the client with the official one and listed what is
missing. This spec moves those items out of the v1 deferred list.

## Done means

Each item is checked by a test the reviewer can rerun: view tests with
`go test ./internal/view`, and calls against the dev server with
`MM_E2E_URL=http://localhost:8065 go test ./internal/e2e -run Parity`.

1. **Click a post to open its thread.** A click on a root post's row
   opens the thread in the RHS, as the webapp's "Click to open threads".
   Links, buttons, reactions and selected text keep their own click.
2. **System posts look like the webapp's.** The system avatar, the name
   "System" and the time head a system post that does not continue a
   group.
3. **Sidebar channel menu.** A ⋮ button on the hovered channel and a
   right click open: Mark as Unread, Favorite, Mute Channel, Move to
   (each category, New Category…), Copy Link, Add Members, Leave Channel.
   A category's header has its own menu: Rename, Delete, Create New
   Category.
4. **Channel menu in the header.** The channel name opens: View Members,
   Add Members, Edit Header, Edit Purpose, Rename, Convert to Private,
   Mute, Archive, Leave. Members shows who is in the channel and removes
   a member.
5. **Create and browse channels.** The sidebar's + creates a public or
   private channel and browses public channels to join.
6. **Integrations.** From the team menu: incoming webhooks (create with a
   channel, copy the URL, delete), outgoing webhooks, slash commands, and
   bot accounts (create, make a token shown once, disable, enable).
7. **Link previews and attachments.** A post shows the server's
   OpenGraph embed (site, title, description, image), image link embeds,
   and message attachments as webhooks and bots send them (color bar,
   pretext, author, title, text, fields, image, footer).

## Round 3 (0.3.0)

Checked by `go test ./internal/view` (round3_test.go) and, against the dev
server, `TestParity2API` and `TestRound3UI` in `internal/e2e`.

1. Search from the top bar, with files; Recent Mentions and Saved
   messages; pinned messages of a channel. Results show in the RHS.
2. Threads: the followed threads of the team, their unread replies and
   mentions, follow and unfollow, mark read.
3. Post menu: Forward, Follow Thread, Mark as Unread, Remind, Save, Pin,
   Edit History. Pinned and Saved show above a post.
4. Profile popover on a post's avatar and name, with Message.
5. Emoji picker with categories, recent emoji, skin tones and custom
   emoji; custom emoji in reactions.
6. Slash commands complete in the composer and run on the server.
7. File preview: images and text or code in the window, other files in
   Quick Look.
8. Group messages, closing a DM or GM, a DM dialog with recent people.
9. Open a channel or a thread in a window of its own.
10. Settings: notifications (desktop, threads, sound, channel-wide
    mentions, first name, keywords), display (own themes, soft dark
    ones; clock; name format; compact; link previews; click to open
    threads), sidebar (unreads first, DM limit), security (username,
    email, password, MFA, sessions, personal access tokens), profile
    picture. Notification preferences of a channel.
11. Sidebar: drag channels and categories, sort and mute a category,
    collapse synced with the server, Add Channels, Invite Members.
12. Channels: channel admins, archived channels to view or unarchive,
    private to public, an Info panel with About, Members, Files and
    bookmarks (bookmarks need an Enterprise license on the server).

## Out of scope

Interactive buttons and menus in attachments, interactive dialogs,
custom emoji upload, push and email notification settings.
