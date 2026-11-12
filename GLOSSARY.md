# Glossary

**Server.** One Mattermost installation the user added, identified by its
base URL. The client holds many servers; each has its own session and data.

**Session.** The login on one server: the user ID and the auth token the
server issued. The token lives in the macOS Keychain, never in a file.

**Team.** A Mattermost team on a server. Channels belong to a team, except
direct and group messages, which belong to no team.

**Channel.** A conversation: public (`O`), private (`P`), direct message
(`D`, two users) or group message (`G`, three to eight users).
Avoid "room" and "conversation".

**DM / GM.** Direct message channel and group message channel.

**Category.** A sidebar section the server defines per user and team
(Favorites, Channels, Direct Messages, and user-made ones). The client shows
categories as the server sends them and does not invent its own grouping.

**Channel member.** The user's membership of a channel. It carries the read
state (`last_viewed_at`, `msg_count`, `mention_count`) and the notify props.

**Unread.** A channel has messages the user has not viewed. **Mention** is an
unread post that names the user, or any post in a DM. The Dock badge shows
mentions as a count and plain unreads as a dot.

**Post.** One message in a channel. Avoid "message" in code; use it only in UI text.

**Root post / reply / thread.** A reply has a `root_id`. A root post and its
replies make a thread.

**RHS.** Right-hand side panel, as the Mattermost webapp calls it. It shows a
thread in v1.

**Store.** The client's in-memory model of one server's data, changed only by
the reducer on the main thread.

**Event.** A typed change to a store, parsed from a WebSocket message or an
API response at the boundary.

**Reducer.** The single function that applies an event to a store.
