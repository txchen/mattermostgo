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

## Out of scope

Interactive buttons and menus in attachments, closing DMs from the
sidebar (needs the `direct_channel_show` preferences, which the client
does not read yet), channel roles (making a member a channel admin),
and the Threads view.
