# Design

Terms follow [GLOSSARY.md](../GLOSSARY.md). Decisions are in [docs/adr](adr/).

## Shape

```
cmd/mattermostgo/     main: app setup, window, deep links, single instance
internal/mm/          the boundary: REST client, WebSocket, JSON -> typed values and events
internal/store/       per-server domain model and the reducer; pure Go, no mygo import
internal/markdown/    Mattermost markdown on top of mygo ui/markdown (@mention, ~channel, :emoji:)
internal/config/      server list file, Keychain tokens, window and UI prefs
internal/app/         wires servers, stores and windows; owns the main-thread loop of events
internal/view/        mygo ui views: top bar, team strip, sidebar, post list, composer, RHS, dialogs
internal/media/       image fetch, disk cache, decode and downscale off the main thread
```

`internal/store` imports nothing from mygo or the network, so its tests run
as plain `go test`. `internal/mm` is the only package that sees JSON.

## Data model

```go
type ServerID string          // stable local ID, not the URL
type UserID, TeamID, ChannelID, PostID, FileID string  // distinct types, never mixed

type Server struct {          // persisted in config, without the token
    ID   ServerID
    Name string
    URL  *url.URL
}

type Store struct {           // one per server, main thread only
    Me          UserID
    Teams       map[TeamID]*Team
    TeamOrder   []TeamID
    Channels    map[ChannelID]*Channel
    Members     map[ChannelID]*ChannelMember   // read state and notify props
    Categories  map[TeamID][]*Category          // as the server orders them
    Users       map[UserID]*User
    Statuses    map[UserID]Status
    Posts       map[PostID]*Post
    Timelines   map[ChannelID]*Timeline         // loaded posts of a channel
    Threads     map[PostID]*Timeline            // loaded replies of a root post
    Typing      map[ChannelID][]Typer           // expires; see timers below
    Conn        ConnState                       // Connecting | Online | Offline{Retry time.Time}
}

type Timeline struct {
    Order      []PostID   // oldest first
    HasOlder   bool       // more posts exist before Order[0]
    Loading    bool
}
```

Unread and mention counts are computed from `Channels` (total message
count) and `Members` (viewed count, mention count). They are never stored
twice, so the badge, the sidebar and the team strip cannot disagree.

Session state is an explicit sum type, not flags:

```go
type Session interface{ isSession() }
type LoggedOut struct{ Err string }                    // shows the login form
type NeedsMFA  struct{ LoginID, Password, Err string } // shows the MFA field
type LoggingIn struct{ Prev Session }                  // a request or the Keychain is on its way
type LoggedIn  struct{ Client *mm.Client }             // the client holds the token
```

The Keychain can block on a permission prompt, so it is read and
written off the main thread (found running the app: a rebuilt, unsigned
binary made the first read wait for the user while frames kept drawing).

## Data flow

1. `internal/mm` issues requests and reads the WebSocket on goroutines.
   It decodes each response or WebSocket message into a typed `store.Event`
   (`PostCreated`, `PostEdited`, `ChannelViewed`, `ReactionAdded`, ...).
   Unknown WebSocket events are dropped at the boundary.
2. It sends the event with `win.Update(func() { store.Apply(ev) })`.
3. `store.Apply` is the reducer, one switch over event types, and the only
   place store fields change.
4. The view reads the store and draws. User actions call `internal/app`,
   which starts requests; results come back through step 1.

Optimistic sends: a new post enters the timeline with a client
`pending_post_id` and state `Sending`; the server's `posted` event with the
same pending ID replaces it. A failed send stays with a retry action.

## Resource rules

These are requirements, checked in review.

- **No polling.** Freshness comes from the WebSocket only. On disconnect,
  reconnect with exponential backoff and jitter (1 s up to 5 min), and
  again at once on wake from sleep or network change. After a reconnect,
  refetch the open channel and unread counts once.
- **No idle frames.** No view requests frames in a still state. Timers
  exist only while they have work: a typing indicator schedules its own
  expiry with `c.After`, and the clock of "5 min ago" labels ticks once a
  minute only while such a label is on screen.
- **Lazy everything.** Fetch a channel's posts when it is first opened, 60
  at a time, older pages when the list scrolls near the top. Fetch users
  and statuses in batches only for IDs on screen. Fetch thumbnails, not
  full images, until the user opens one.
- **Bounded memory.** Keep timelines for the open channel and the 10 most
  recently viewed per server; drop the rest (LRU). Decoded images are
  downscaled to display size times the screen scale and kept in an LRU
  bounded by bytes; the encoded files sit in a disk cache under the app's
  cache directory.
- **Off the main thread.** JSON decoding, Markdown parsing of new posts,
  and image decoding happen before `win.Update`. The main thread only
  swaps in results.
- **Inactive servers stay connected, but quiet.** Their WebSocket stays
  open for notifications and badges. Their events update counts only;
  posts of channels not loaded are not kept.
- **Small binary.** No dependencies beyond mygo and the standard library
  unless a module earns its size. Emoji images are not bundled; system
  emoji render through the system font, and custom emoji load from the server.

## Notifications and badge

On `posted` for a channel the user is not looking at (or when the app is
not focused), decide whether to notify from the channel member's notify
props and the user's desktop setting (all, mentions, none), and mute. A
notification click focuses the window and opens the channel. The Dock badge
shows the total mention count across servers, or a dot when only unreads
exist (`App.Dock.SetBadge`). Marking a channel viewed posts
`/channels/members/me/view` once when it is opened and when new posts
arrive while it is in view and the window is focused.

## Testing

- `internal/store`: table tests of the reducer, plus generated event
  sequences checking invariants (no duplicate post IDs in a timeline,
  replies only in their root's thread, counts never negative).
- `internal/mm`: decoding tests against recorded JSON fixtures from a real
  server, and a fake server (`httptest`) for login and WebSocket reconnects.
- `internal/view`: mygo view tests without a window (`ui.Tester`).
- End to end on a Mac against a local Mattermost server in Docker (issue 03).
- Resource checks on a Mac: idle CPU over 60 s, RSS with 1 and 3 servers,
  package size, compared with the official client on the same servers.
