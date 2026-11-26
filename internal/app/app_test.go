package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/txchen/mattermostgo/internal/config"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// Opening an unread channel that already has a cached timeline still
// fetches the newest page from the server, so missed posts appear.
func TestOpenUnreadChannelRefetchesStaleTimeline(t *testing.T) {
	var postFetches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/channels/b/posts":
			postFetches.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": []string{"p2", "p1"},
				"posts": map[string]any{
					"p1": map[string]any{"id": "p1", "channel_id": "b", "user_id": "u1", "create_at": 1000, "message": "old"},
					"p2": map[string]any{"id": "p2", "channel_id": "b", "user_id": "u1", "create_at": 2000, "message": "new"},
				},
			})
		case "/api/v4/channels/b/stats":
			_ = json.NewEncoder(w).Encode(map[string]any{"member_count": 2})
		case "/api/v4/channels/members/me/view":
			w.WriteHeader(http.StatusOK)
		case "/api/v4/users/ids", "/api/v4/users/status/ids":
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	queue := make(chan func(), 32)
	a := &App{
		Run:        func(fn func()) { queue <- fn },
		HC:         srv.Client(),
		ConfigPath: filepath.Join(t.TempDir(), "servers.json"),
		Focused:    func() bool { return true },
	}
	st := store.New()
	st.Apply(store.Loaded{
		Me:    store.User{ID: "me", Username: "me"},
		Teams: []store.Team{{ID: "t1", Name: "team"}},
		Channels: []store.Channel{
			{ID: "a", TeamID: "t1", Type: store.Open, TotalMsgCount: 1},
			{ID: "b", TeamID: "t1", Type: store.Open, TotalMsgCount: 2},
		},
		Members: []store.ChannelMember{
			{ChannelID: "a", MsgCount: 1},
			{ChannelID: "b", MsgCount: 1},
		},
	})
	st.Apply(store.PostsLoaded{
		Channel: "b",
		Posts:   []store.Post{{ID: "p1", ChannelID: "b", UserID: "u1", CreateAt: 1000, Message: "old"}},
	})
	s := &Server{
		Config:  config.Server{ID: "s1", Name: "test", URL: srv.URL},
		URL:     base,
		Session: LoggedIn{Client: mm.New(base, "tok", srv.Client())},
		Store:   st,
		Ready:   true,
		Team:    "t1",
		Channel: "a",
	}
	a.Servers = []*Server{s}
	a.Current = s

	a.OpenChannel(s, "b")

	deadline := time.After(3 * time.Second)
	for !slices.Equal(st.Timelines["b"].Order, []store.PostID{"p1", "p2"}) {
		select {
		case fn := <-queue:
			fn()
		case <-deadline:
			t.Fatalf("timeline order = %v, want [p1 p2]", st.Timelines["b"].Order)
		}
	}
	if postFetches.Load() != 1 {
		t.Fatalf("postFetches = %d, want 1", postFetches.Load())
	}

	// Opening the same channel again while already read does not refetch.
	a.OpenChannel(s, "b")
	if postFetches.Load() != 1 || st.Timelines["b"].Loading {
		t.Fatalf("unexpected refetch on read channel: fetches=%d loading=%v", postFetches.Load(), st.Timelines["b"].Loading)
	}
}
