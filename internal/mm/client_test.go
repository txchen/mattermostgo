package mm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/txchen/mattermostgo/internal/store"
)

var ctx = context.Background()

// An account with MFA asks for the code first, rejects a wrong one, and
// logs in with the right one.
func TestLoginMFA(t *testing.T) {
	f := newFakeServer(t)
	f.mfa = true
	if _, _, err := Login(ctx, http.DefaultClient, f.base(), "alice", "pw", ""); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("no code: %v", err)
	}
	_, _, err := Login(ctx, http.DefaultClient, f.base(), "alice", "pw", "000000")
	var ae *APIError
	if !errors.As(err, &ae) || ae.ID != "api.user.check_user_mfa.bad_code.app_error" {
		t.Fatalf("wrong code: %v", err)
	}
	c, me, err := Login(ctx, http.DefaultClient, f.base(), "alice", "pw", "123456")
	if err != nil || c.Token() != "tok" || me.ID != "alice1" || me.FullName() != "Alice Liddell" {
		t.Fatalf("login: %v %+v", err, me)
	}
}

// A wrong password is the server's message, not an expired session.
func TestLoginWrongPassword(t *testing.T) {
	f := newFakeServer(t)
	_, _, err := Login(ctx, http.DefaultClient, f.base(), "alice", "nope", "")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Message != "Enter a valid email or username and/or password." {
		t.Fatalf("got %v", err)
	}
}

// A revoked token reads as an expired session.
func TestExpiredToken(t *testing.T) {
	f := newFakeServer(t)
	c := New(f.base(), "old", http.DefaultClient)
	if _, err := c.Me(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

func TestLoad(t *testing.T) {
	f := newFakeServer(t)
	c := New(f.base(), "tok", http.DefaultClient)
	ev, err := c.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Me.ID != "alice1" || len(ev.Teams) != 1 || len(ev.Channels) != 2 || len(ev.Members) != 2 {
		t.Fatalf("%+v", ev)
	}
	s := store.New()
	s.Apply(ev)
	if !s.Unread("c1") || s.Mentions("c1") != 1 || s.Unread("d1") {
		t.Errorf("unread c1 %v mentions %d unread d1 %v", s.Unread("c1"), s.Mentions("c1"), s.Unread("d1"))
	}
	if !s.Members["d1"].Muted() || s.Channels["d1"].DMPartner("alice1") != "bob1" {
		t.Errorf("muted %v partner %q", s.Members["d1"].Muted(), s.Channels["d1"].DMPartner("alice1"))
	}
	cats, err := c.Categories(ctx, "t1")
	if err != nil || len(cats) != 2 || cats[0].Type != store.CategoryChannels || !slices.Equal(cats[1].Channels, []store.ChannelID{"d1"}) {
		t.Errorf("categories %v %+v", err, cats)
	}
}

// A page of posts skips deleted posts and reads files, reactions and props.
func TestPosts(t *testing.T) {
	f := newFakeServer(t)
	c := New(f.base(), "tok", http.DefaultClient)
	page, err := c.Posts(ctx, "c1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Posts) != 2 || page.Older || page.HasOlder {
		t.Fatalf("%+v", page)
	}
	p := page.Posts[0]
	if p.ID != "p2" || p.EditAt != 25 || !p.Props.FromBot || len(p.Files) != 1 || !p.Files[0].IsImage() || p.Files[0].Width != 640 || len(p.Reactions) != 1 || p.Reactions[0].EmojiName != "+1" {
		t.Errorf("%+v", p)
	}
	older, err := c.Posts(ctx, "c1", "p1")
	if err != nil || !older.Older || len(older.Posts) != 1 {
		t.Errorf("older %v %+v", err, older)
	}
}

func TestCreatePost(t *testing.T) {
	f := newFakeServer(t)
	c := New(f.base(), "tok", http.DefaultClient)
	p, err := c.CreatePost(ctx, NewPost{ChannelID: "c1", Message: "yo", PendingID: "alice1:1"})
	if err != nil || p.ID != "new1" || p.PendingID != "alice1:1" || p.Message != "yo" {
		t.Fatalf("%v %+v", err, p)
	}
}

func wsEvent(event string, data map[string]any, broadcast map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"event": event, "data": data, "broadcast": broadcast, "seq": 1})
	return b
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// WebSocket messages as the server writes them become store events.
func TestDecodeEvent(t *testing.T) {
	now := time.Unix(1000, 0)
	post := map[string]any{"id": "p9", "channel_id": "c1", "user_id": "bob1", "create_at": 99, "message": "hey @alice"}
	cases := []struct {
		name string
		raw  []byte
		want store.Event
		f    fetch
	}{
		{"posted mentioning me", wsEvent("posted", map[string]any{"post": jsonString(post), "mentions": jsonString([]string{"alice1"}), "channel_type": "O"}, nil),
			store.PostCreated{Post: store.Post{ID: "p9", ChannelID: "c1", UserID: "bob1", CreateAt: 99, Message: "hey @alice"}, Mentioned: true}, fetch{}},
		{"posted", wsEvent("posted", map[string]any{"post": jsonString(post)}, nil),
			store.PostCreated{Post: store.Post{ID: "p9", ChannelID: "c1", UserID: "bob1", CreateAt: 99, Message: "hey @alice"}}, fetch{}},
		{"post deleted", wsEvent("post_deleted", map[string]any{"post": jsonString(post)}, nil), store.PostDeleted{ID: "p9", Channel: "c1"}, fetch{}},
		{"reaction", wsEvent("reaction_added", map[string]any{"reaction": jsonString(map[string]any{"user_id": "bob1", "post_id": "p9", "emoji_name": "tada", "create_at": 5})}, nil),
			store.ReactionAdded{Post: "p9", Reaction: store.Reaction{UserID: "bob1", EmojiName: "tada", CreateAt: 5}}, fetch{}},
		{"typing", wsEvent("typing", map[string]any{"user_id": "bob1", "parent_id": ""}, map[string]any{"channel_id": "c1"}),
			store.Typing{Channel: "c1", User: "bob1", At: now}, fetch{}},
		{"viewed elsewhere", wsEvent("multiple_channels_viewed", map[string]any{"channel_times": map[string]int64{"c1": 77}}, map[string]any{"user_id": "alice1"}),
			store.ChannelsViewed{Times: map[store.ChannelID]int64{"c1": 77}}, fetch{}},
		{"status", wsEvent("status_change", map[string]any{"status": "away", "user_id": "bob1"}, nil),
			store.StatusesChanged{Statuses: map[store.UserID]store.Status{"bob1": store.Away}}, fetch{}},
		{"removed from channel", wsEvent("user_removed", map[string]any{"channel_id": "c1", "remover_id": "bob1"}, map[string]any{"user_id": "alice1"}),
			store.ChannelRemoved{Channel: "c1"}, fetch{}},
		{"someone else removed", wsEvent("user_removed", map[string]any{"user_id": "carol1"}, map[string]any{"channel_id": "c1"}), nil, fetch{}},
		{"added to channel", wsEvent("user_added", map[string]any{"user_id": "alice1", "team_id": "t1"}, map[string]any{"channel_id": "c7"}), nil, fetch{kind: fetchChannel, channel: "c7"}},
		{"new DM", wsEvent("direct_added", map[string]any{"creator_id": "bob1", "teammate_id": "alice1"}, map[string]any{"channel_id": "d9"}), nil, fetch{kind: fetchChannel, channel: "d9"}},
		{"categories", wsEvent("sidebar_category_updated", map[string]any{"updatedCategories": "[]"}, map[string]any{"team_id": "t1", "user_id": "alice1"}), nil, fetch{kind: fetchCategories, team: "t1"}},
		{"user updated", wsEvent("user_updated", map[string]any{"user": map[string]any{"id": "bob1", "username": "bobby"}}, nil),
			store.UsersLoaded{Users: []store.User{{ID: "bob1", Username: "bobby"}}}, fetch{}},
		{"unknown", wsEvent("plugin_statuses_changed", nil, nil), nil, fetch{}},
	}
	for _, tc := range cases {
		events, f, err := decodeEvent(tc.raw, "alice1", now)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		var got store.Event
		if len(events) == 1 {
			got = events[0]
		}
		if jsonString(got) != jsonString(tc.want) || f != tc.f {
			t.Errorf("%s:\n got %#v %+v\nwant %#v %+v", tc.name, got, f, tc.want, tc.f)
		}
	}
}

// The socket goes online on hello, passes events on, and after the
// server drops it, reconnects and says it reconnected.
func TestSocketReconnects(t *testing.T) {
	f := newFakeServer(t)
	events := make(chan store.Event, 64)
	online := make(chan bool, 4)
	s := &Socket{Client: New(f.base(), "tok", http.DefaultClient), HC: http.DefaultClient, Me: "alice1",
		Emit: func(ev store.Event) { events <- ev }, Online: func(re bool) { online <- re }}
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.Run(c)

	sock := <-f.sockets
	sock.send(map[string]any{"event": "hello", "data": map[string]any{"connection_id": "x"}, "broadcast": map[string]any{"user_id": "alice1"}, "seq": 0})
	if re := <-online; re {
		t.Fatal("first connection reported as reconnect")
	}
	s.Typing("c1", "")
	msg, err := sock.read()
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	json.Unmarshal(msg, &req)
	if req["action"] != "user_typing" {
		t.Errorf("sent %s", msg)
	}
	sock.send(map[string]any{"status": "OK", "seq_reply": 1})
	sock.send(map[string]any{"event": "posted", "data": map[string]any{"post": jsonString(map[string]any{"id": "p1", "channel_id": "c1"})}, "broadcast": map[string]any{}, "seq": 1})
	if !waitFor(events, func(ev store.Event) bool { _, ok := ev.(store.PostCreated); return ok }) {
		t.Fatal("no post")
	}
	sock.conn.Close()
	if !waitFor(events, func(ev store.Event) bool { _, ok := ev.(store.ConnChanged).Conn.(store.ConnOffline); return ok }) {
		t.Fatal("not offline")
	}
	s.Wake()
	sock = <-f.sockets
	sock.send(map[string]any{"event": "hello", "data": map[string]any{}, "broadcast": map[string]any{}, "seq": 0})
	if re := <-online; !re {
		t.Error("reconnect reported as first connection")
	}
}

// A socket whose session the server rejects stops and reports it.
func TestSocketExpired(t *testing.T) {
	f := newFakeServer(t)
	f.rejectWS = true
	expired := make(chan struct{})
	s := &Socket{Client: New(f.base(), "tok", http.DefaultClient), HC: http.DefaultClient, Emit: func(store.Event) {}, Expired: func() { close(expired) }}
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	select {
	case <-expired:
	case <-time.After(5 * time.Second):
		t.Fatal("not expired")
	}
	<-done
}

func waitFor(events chan store.Event, ok func(store.Event) bool) bool {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			if func() (r bool) {
				defer func() { recover() }()
				return ok(ev)
			}() {
				return true
			}
		case <-timeout:
			return false
		}
	}
}
