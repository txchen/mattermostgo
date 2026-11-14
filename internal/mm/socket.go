package mm

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/txchen/mattermostgo/internal/store"
)

// wsMessage is a message the server sends over the WebSocket.
type wsMessage struct {
	Event     string                     `json:"event"`
	Data      map[string]json.RawMessage `json:"data"`
	Broadcast struct {
		UserID    string `json:"user_id"`
		ChannelID string `json:"channel_id"`
		TeamID    string `json:"team_id"`
	} `json:"broadcast"`
	Seq int64 `json:"seq"`
}

// str reads a data field that is a string, or a JSON value written as one.
func (m *wsMessage) str(key string) string {
	raw := m.Data[key]
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// decode reads a data field holding JSON, as a string or as an object.
func (m *wsMessage) decode(key string, v any) error {
	raw := m.Data[key]
	var s string
	if json.Unmarshal(raw, &s) == nil {
		raw = []byte(s)
	}
	return json.Unmarshal(raw, v)
}

// fetch is a fact the WebSocket only hints at, which a request must get.
type fetch struct {
	kind    fetchKind
	channel store.ChannelID
	team    store.TeamID
}

type fetchKind uint8

const (
	fetchNone fetchKind = iota
	fetchChannel
	fetchCategories
	fetchThreads
	fetchAll
)

// decodeEvent turns a WebSocket message into store events, or into a
// fetch for messages that carry only IDs. Unknown messages give neither.
func decodeEvent(raw []byte, me store.UserID, now time.Time) ([]store.Event, fetch, error) {
	var m wsMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fetch{}, err
	}
	one := func(ev store.Event) ([]store.Event, fetch, error) { return []store.Event{ev}, fetch{}, nil }
	need := func(f fetch) ([]store.Event, fetch, error) { return nil, f, nil }
	channel := store.ChannelID(m.Broadcast.ChannelID)
	switch m.Event {
	case "posted", "post_edited":
		var p wirePost
		if err := m.decode("post", &p); err != nil {
			return nil, fetch{}, err
		}
		if m.Event == "post_edited" {
			return one(store.PostEdited{Post: p.post()})
		}
		var mentions []string
		_ = m.decode("mentions", &mentions)
		return one(store.PostCreated{Post: p.post(), Mentioned: slices.Contains(mentions, string(me))})
	case "post_deleted":
		var p wirePost
		if err := m.decode("post", &p); err != nil {
			return nil, fetch{}, err
		}
		return one(store.PostDeleted{ID: store.PostID(p.ID), Channel: store.ChannelID(p.ChannelID)})
	case "reaction_added", "reaction_removed":
		var r wireReaction
		if err := m.decode("reaction", &r); err != nil {
			return nil, fetch{}, err
		}
		if m.Event == "reaction_added" {
			return one(store.ReactionAdded{Post: store.PostID(r.PostID), Reaction: r.reaction()})
		}
		return one(store.ReactionRemoved{Post: store.PostID(r.PostID), User: store.UserID(r.UserID), EmojiName: r.EmojiName})
	case "typing":
		return one(store.Typing{Channel: channel, User: store.UserID(m.str("user_id")), At: now})
	case "status_change":
		return one(store.StatusesChanged{Statuses: map[store.UserID]store.Status{store.UserID(m.str("user_id")): store.Status(m.str("status"))}})
	case "multiple_channels_viewed":
		var times map[string]int64
		if err := m.decode("channel_times", &times); err != nil {
			return nil, fetch{}, err
		}
		ev := store.ChannelsViewed{Times: map[store.ChannelID]int64{}}
		for id, at := range times {
			ev.Times[store.ChannelID(id)] = at
		}
		return one(ev)
	case "channel_member_updated":
		var w wireMember
		if err := m.decode("channelMember", &w); err != nil {
			return nil, fetch{}, err
		}
		return one(store.MemberUpdated{Member: w.member()})
	case "channel_updated":
		var w wireChannel
		if err := m.decode("channel", &w); err != nil {
			return need(fetch{kind: fetchChannel, channel: channel})
		}
		return need(fetch{kind: fetchChannel, channel: store.ChannelID(w.ID)})
	case "open_dialog":
		var w wireDialog
		if err := m.decode("dialog", &w); err != nil {
			return nil, fetch{}, err
		}
		return one(store.DialogOpened{Dialog: w.dialog()})
	case "ephemeral_message":
		var p wirePost
		if err := m.decode("post", &p); err != nil {
			return nil, fetch{}, err
		}
		post := p.post()
		post.Ephemeral = true
		return one(store.PostCreated{Post: post})
	case "preferences_changed", "preferences_deleted":
		var ws []wirePref
		if err := m.decode("preferences", &ws); err != nil {
			return nil, fetch{}, err
		}
		ev := store.PrefsChanged{Deleted: m.Event == "preferences_deleted"}
		for _, w := range ws {
			ev.Prefs = append(ev.Prefs, w.pref())
		}
		return one(ev)
	case "thread_updated":
		var w wireThread
		if err := m.decode("thread", &w); err != nil {
			return nil, fetch{}, err
		}
		return one(store.ThreadChanged{Thread: w.state(store.TeamID(m.Broadcast.TeamID))})
	case "thread_follow_changed":
		if m.str("state") == "false" {
			return one(store.ThreadChanged{Thread: store.ThreadState{Root: store.PostID(m.str("thread_id"))}})
		}
		return need(fetch{kind: fetchThreads, team: store.TeamID(m.Broadcast.TeamID)})
	case "thread_read_changed":
		num := func(k string) int64 { n, _ := strconv.ParseInt(m.str(k), 10, 64); return n }
		return one(store.ThreadRead{Root: store.PostID(m.str("thread_id")), LastViewedAt: num("timestamp"), UnreadReplies: num("unread_replies"), UnreadMentions: num("unread_mentions")})
	case "post_unread", "channel_converted":
		id := store.ChannelID(m.str("channel_id"))
		if id == "" {
			id = channel
		}
		return need(fetch{kind: fetchChannel, channel: id})
	case "channel_deleted":
		return one(store.ChannelRemoved{Channel: store.ChannelID(m.str("channel_id"))})
	case "channel_created":
		return need(fetch{kind: fetchChannel, channel: store.ChannelID(m.str("channel_id"))})
	case "direct_added", "group_added":
		return need(fetch{kind: fetchChannel, channel: channel})
	case "user_added":
		if store.UserID(m.str("user_id")) == me {
			return need(fetch{kind: fetchChannel, channel: channel})
		}
	case "user_removed":
		if store.UserID(m.Broadcast.UserID) == me {
			if id := m.str("channel_id"); id != "" {
				return one(store.ChannelRemoved{Channel: store.ChannelID(id)})
			}
		}
	case "user_updated":
		var u wireUser
		if err := m.decode("user", &u); err == nil && u.ID != "" {
			return one(store.UsersLoaded{Users: []store.User{u.user()}})
		}
	case "sidebar_category_created", "sidebar_category_updated", "sidebar_category_deleted", "sidebar_category_order_updated":
		team := m.Broadcast.TeamID
		if team == "" {
			team = m.str("team_id")
		}
		if team == "" {
			return need(fetch{kind: fetchAll})
		}
		return need(fetch{kind: fetchCategories, team: store.TeamID(team)})
	case "added_to_team", "leave_team", "update_team":
		return need(fetch{kind: fetchAll})
	}
	return nil, fetch{}, nil
}

// Socket keeps a server's WebSocket open: it reconnects with backoff, and
// sends what it receives as store events.
type Socket struct {
	Client *Client
	HC     *http.Client
	Me     store.UserID
	// Emit receives events on the socket's goroutine.
	Emit func(store.Event)
	// Online runs after each successful connection; reconnected is false
	// for the first one. Missed events are not replayed, so a reconnect
	// must fetch again what is on screen.
	Online func(reconnected bool)
	// Expired runs when the server rejects the session.
	Expired func()

	mu    sync.Mutex
	conn  *wsConn
	seq   int64
	wake  chan struct{}
	start sync.Once
}

// debug logs the WebSocket messages when MMGO_DEBUG is set.
var debug = os.Getenv("MMGO_DEBUG") != ""

const (
	minBackoff = time.Second
	maxBackoff = 5 * time.Minute
	// silence is how long a connection may go without a frame; the
	// server pings every 60 s.
	silence = 100 * time.Second
)

// Run connects until ctx ends.
func (s *Socket) Run(ctx context.Context) {
	s.start.Do(func() { s.wake = make(chan struct{}, 1) })
	s.HC = http1Only(s.HC)
	backoff := minBackoff
	connected := false
	for ctx.Err() == nil {
		s.Emit(store.ConnChanged{Conn: store.ConnConnecting{}})
		start := time.Now()
		err := s.session(ctx, &connected)
		if debug {
			log.Printf("websocket session ended: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			s.Emit(store.ConnChanged{Conn: store.ConnOffline{Err: err.Error()}})
			if s.Expired != nil {
				s.Expired()
			}
			return
		}
		if time.Since(start) > time.Minute {
			backoff = minBackoff
		}
		wait := backoff/2 + rand.N(backoff)
		backoff = min(backoff*2, maxBackoff)
		msg := "disconnected"
		if err != nil {
			msg = err.Error()
		}
		s.Emit(store.ConnChanged{Conn: store.ConnOffline{Retry: time.Now().Add(wait), Err: msg}})
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		case <-s.wake:
			backoff = minBackoff
		}
	}
}

// Wake reconnects at once if the socket waits to retry, as after the Mac
// wakes from sleep or the network changes.
func (s *Socket) Wake() {
	s.start.Do(func() { s.wake = make(chan struct{}, 1) })
	select {
	case s.wake <- struct{}{}:
	default:
	}
	s.mu.Lock()
	c := s.conn
	s.mu.Unlock()
	if c != nil {
		// A connection that slept may be dead without knowing it; a ping
		// gets an answer or an error soon.
		_ = c.write(opPing, nil)
	}
}

func (s *Socket) session(ctx context.Context, connected *bool) error {
	u := *s.Client.base
	u.Path += "/api/v4/websocket"
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	conn, err := dialWS(dialCtx, s.HC, u.String(), http.Header{"Authorization": {"Bearer " + s.Client.token}})
	cancel()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.conn = nil
		s.mu.Unlock()
		conn.Close()
	}()
	alive := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTimer(silence)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				conn.Close()
				return
			case <-alive:
				t.Reset(silence)
			case <-t.C:
				conn.Close()
				return
			}
		}
	}()
	for {
		raw, err := conn.ReadMessage(func() {
			select {
			case alive <- struct{}{}:
			default:
			}
		})
		if err != nil {
			return err
		}
		var head struct {
			Event string `json:"event"`
		}
		if json.Unmarshal(raw, &head) != nil || head.Event == "" {
			continue // a reply to a request of ours
		}
		if head.Event == "hello" {
			s.Emit(store.ConnChanged{Conn: store.ConnOnline{}})
			if s.Online != nil {
				go s.Online(*connected)
			}
			*connected = true
			continue
		}
		events, f, err := decodeEvent(raw, s.Me, time.Now())
		if debug {
			log.Printf("websocket %s: %d events, fetch %v, err %v", head.Event, len(events), f.kind, err)
		}
		if err != nil {
			continue
		}
		for _, ev := range events {
			s.Emit(ev)
		}
		if f.kind != fetchNone {
			go s.fetch(ctx, f)
		}
	}
}

func (s *Socket) fetch(ctx context.Context, f fetch) {
	switch f.kind {
	case fetchChannel:
		ch, err := s.Client.Channel(ctx, f.channel)
		if err != nil {
			return
		}
		ev := store.ChannelUpserted{Channel: ch}
		if m, err := s.Client.Member(ctx, f.channel); err == nil {
			ev.Member = &m
		}
		s.Emit(ev)
		if ch.TeamID != "" {
			s.fetch(ctx, fetch{kind: fetchCategories, team: ch.TeamID})
		}
	case fetchCategories:
		if cats, err := s.Client.Categories(ctx, f.team); err == nil {
			s.Emit(store.CategoriesLoaded{Team: f.team, Categories: cats})
		}
	case fetchThreads:
		if f.team == "" || s.Me == "" {
			return
		}
		if ev, err := s.Client.Threads(ctx, s.Me, f.team); err == nil {
			s.Emit(ev)
		}
	case fetchAll:
		if s.Online != nil {
			s.Online(true)
		}
	}
}

// Typing tells the server the user is typing in a channel or thread.
// ActiveStatus tells the server whether the user uses the computer, as
// the official desktop app does: the server keeps a user it hears nothing
// from online for its away timeout only, then makes them away. It
// changes no status the user set by hand.
func (s *Socket) ActiveStatus(active bool) {
	s.send("user_update_active_status", map[string]any{"user_is_active": active, "manual": false})
}

func (s *Socket) Typing(ch store.ChannelID, parent store.PostID) {
	s.send("user_typing", map[string]any{"channel_id": ch, "parent_id": parent})
}

func (s *Socket) send(action string, data any) {
	s.mu.Lock()
	c := s.conn
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	if c == nil {
		return
	}
	b, _ := json.Marshal(map[string]any{"seq": seq, "action": action, "data": data})
	_ = c.WriteText(b)
}
