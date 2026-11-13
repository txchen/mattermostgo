package view

import (
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/config"
	"github.com/txchen/mattermostgo/internal/media"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

type memSecrets struct{ m map[string][]byte }

func (s *memSecrets) Get(k string) ([]byte, error) { return s.m[k], nil }
func (s *memSecrets) Set(k string, v []byte) error { s.m[k] = v; return nil }
func (s *memSecrets) Delete(k string) error        { delete(s.m, k); return nil }

// harness is a view of one logged-in server whose API answers with the
// handler; work that comes back to the main thread waits in a queue.
type harness struct {
	t     *testing.T
	app   *app.App
	view  *View
	srv   *app.Server
	tt    *ui.Tester
	mu    sync.Mutex
	queue []func()
	posts chan map[string]any
	// searches counts the user searches the view asked the server for.
	searches int
	// calls are the requests no handler took; replies answers them, by
	// method and path.
	calls   []call
	replies map[string]string
}

type call struct {
	key  string
	body any
}

// called waits for a request the view made, and returns its body.
func (h *harness) called(key string) map[string]any {
	h.t.Helper()
	b, _ := h.calledWith(key).(map[string]any)
	return b
}

func (h *harness) calledWith(key string) any {
	h.t.Helper()
	var body any
	found := false
	h.settle(func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, c := range h.calls {
			if c.key == key {
				body, found = c.body, true
			}
		}
		return found
	})
	return body
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, posts: make(chan map[string]any, 8), replies: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v4/posts", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		h.posts <- body
		if strings.Contains(body["message"].(string), "fail") {
			w.WriteHeader(500)
			return
		}
		body["id"] = "srv-" + body["pending_post_id"].(string)
		body["user_id"] = "me"
		body["create_at"] = time.Now().UnixMilli()
		json.NewEncoder(w).Encode(body)
	})
	mux.HandleFunc("GET /api/v4/posts/p1/thread", func(w http.ResponseWriter, r *http.Request) {
		at := time.Now().Add(-time.Hour).UnixMilli()
		json.NewEncoder(w).Encode(map[string]any{"order": []string{"r1", "p1"}, "posts": map[string]any{
			"p1": map[string]any{"id": "p1", "channel_id": "town", "user_id": "bob", "create_at": at, "message": "hello **team**"},
			"r1": map[string]any{"id": "r1", "channel_id": "town", "user_id": "bob", "root_id": "p1", "create_at": at + 1000, "message": "an earlier reply"},
		}})
	})
	mux.HandleFunc("GET /api/v4/users/autocomplete", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.searches++
		h.mu.Unlock()
		w.Write([]byte(`{"users":[{"id":"bob","username":"bob"},{"id":"bobby","username":"bobby"}]}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var body any
		json.NewDecoder(r.Body).Decode(&body)
		h.mu.Lock()
		defer h.mu.Unlock()
		key := r.Method + " " + r.URL.Path
		h.calls = append(h.calls, call{key, body})
		if reply, ok := h.replies[key]; ok {
			w.Write([]byte(reply))
			return
		}
		w.WriteHeader(404)
	})
	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	u, _ := url.Parse(hs.URL)
	h.app = &app.App{
		Run:        func(f func()) { h.mu.Lock(); h.queue = append(h.queue, f); h.mu.Unlock() },
		HC:         hs.Client(),
		Secrets:    &memSecrets{m: map[string][]byte{}},
		ConfigPath: filepath.Join(t.TempDir(), "servers.json"),
	}
	h.srv = &app.Server{Config: config.Server{ID: "s1", Name: "Test", URL: hs.URL}, URL: u,
		Session: app.LoggedIn{Client: mm.New(u, "tok", hs.Client())}, Store: store.New(), Ready: true}
	h.app.Servers = []*app.Server{h.srv}
	h.app.Current = h.srv
	h.view = New(h.app, media.New(h.app.Run, "", 1<<20))
	h.app.Error = h.view.ShowError
	h.load()
	h.tt = ui.NewTester(h.view.Build, 1100, 700)
	return h
}

// load fills the store as a server's first load does.
func (h *harness) load() {
	s := h.srv.Store
	s.Apply(store.Loaded{
		Me:    store.User{ID: "me", Username: "me"},
		Teams: []store.Team{{ID: "t1", Name: "eng", DisplayName: "Engineering"}},
		Channels: []store.Channel{
			{ID: "town", TeamID: "t1", Type: store.Open, Name: "town-square", DisplayName: "Town Square", TotalMsgCount: 2},
			{ID: "dev", TeamID: "t1", Type: store.Open, Name: "dev", DisplayName: "Developers", TotalMsgCount: 5},
			{ID: "dm", Type: store.Direct, Name: "bob__me", TotalMsgCount: 1},
		},
		Members: []store.ChannelMember{
			{ChannelID: "town", MsgCount: 2}, {ChannelID: "dev", MsgCount: 3, MentionCount: 2}, {ChannelID: "dm", MsgCount: 1},
		},
	})
	s.Apply(store.UsersLoaded{Users: []store.User{{ID: "bob", Username: "bob"}}})
	s.Apply(store.CategoriesLoaded{Team: "t1", Categories: []store.Category{
		{ID: "c-ch", Type: store.CategoryChannels, DisplayName: "Channels", Channels: []store.ChannelID{"town", "dev"}},
		{ID: "c-dm", Type: store.CategoryDirect, DisplayName: "Direct Messages", Channels: []store.ChannelID{"dm"}},
	}})
	h.srv.Team, h.srv.Channel = "t1", "town"
	s.Apply(store.PostsLoaded{Channel: "town", Posts: []store.Post{
		{ID: "p1", ChannelID: "town", UserID: "bob", CreateAt: time.Now().Add(-time.Hour).UnixMilli(), Message: "hello **team**"},
		{ID: "p2", ChannelID: "town", UserID: "bob", CreateAt: time.Now().Add(-59 * time.Minute).UnixMilli(), Message: "second"},
	}})
}

// settle runs what came back to the main thread until cond holds.
func (h *harness) settle(cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.mu.Lock()
		q := h.queue
		h.queue = nil
		h.mu.Unlock()
		for _, f := range q {
			f()
		}
		h.tt.Frame()
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("not settled; texts %q", h.tt.Texts())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *harness) snapshot(name string) {
	dir := os.Getenv("MM_FRAMES")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, h.tt.Image())
}

// The sidebar shows the server's categories, a channel's mentions, and
// opens the channel clicked.
func TestSidebarOpensChannel(t *testing.T) {
	h := newHarness(t)
	for _, s := range []string{"CHANNELS", "DIRECT MESSAGES", "Developers", "bob", "2", "hello", "team"} {
		if !strings.Contains(strings.Join(h.tt.Texts(), "\n"), s) {
			t.Errorf("no %q in %q", s, h.tt.Texts())
		}
	}
	h.snapshot("sidebar")
	if err := h.tt.Click("Developers"); err != nil {
		t.Fatal(err)
	}
	if h.srv.Channel != "dev" {
		t.Fatalf("open channel %q", h.srv.Channel)
	}
	if h.srv.Store.Mentions("dev") != 0 || h.srv.Store.Unread("dev") {
		t.Error("opening the channel did not mark it read")
	}
}

// Enter sends the draft: the post shows at once, then as the server's.
func TestComposerSends(t *testing.T) {
	h := newHarness(t)
	h.tt.Type("ship it")
	h.tt.Key(0, ui.KeyEnter)
	body := <-h.posts
	if body["message"] != "ship it" || body["channel_id"] != "town" {
		t.Fatalf("sent %v", body)
	}
	if !h.tt.HasText("ship it") {
		t.Fatalf("pending post not shown: %q", h.tt.Texts())
	}
	h.settle(func() bool {
		tl := h.srv.Store.Timelines["town"]
		last := h.srv.Store.Posts[tl.Order[len(tl.Order)-1]]
		return last.State == store.Sent && strings.HasPrefix(string(last.ID), "srv-")
	})
	if *h.view.drafts[draftKey{"s1", "town", ""}] != "" {
		t.Error("draft kept after sending")
	}
}

// A post the server refuses offers a retry.
func TestFailedSendRetries(t *testing.T) {
	h := newHarness(t)
	h.tt.Type("this will fail")
	h.tt.Key(0, ui.KeyEnter)
	<-h.posts
	h.settle(func() bool { return h.tt.HasText("Message failed to send.") })
	if err := h.tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	if b := <-h.posts; b["message"] != "this will fail" {
		t.Errorf("retried %v", b)
	}
}

// Typing @ suggests users, and Enter completes the name.
func TestMentionAutocomplete(t *testing.T) {
	h := newHarness(t)
	h.tt.Type("hi @bo")
	h.settle(func() bool { return h.tt.HasText("@bobby") })
	// The users found arrive once: asking again for each frame they
	// cause made a loop of searches (found in the real window).
	h.settle(func() bool { return true })
	h.tt.Advance(time.Second)
	h.settle(func() bool { return true })
	if h.searches != 1 {
		t.Errorf("%d searches for one prefix", h.searches)
	}
	h.tt.Key(0, ui.KeyEnter)
	// Text typed right after the completion follows it. It came before it
	// when the completion waited for the next frame (found in the real
	// window, where keys arrive faster than frames).
	h.tt.Type("there")
	h.tt.Frame()
	if got := *h.view.drafts[draftKey{"s1", "town", ""}]; got != "hi @bob there" {
		t.Errorf("draft %q", got)
	}
	h.tt.Key(0, ui.KeyEnter)
	if b := <-h.posts; b["message"] != "hi @bob there" {
		t.Errorf("sent %v", b)
	}
	// The same word completes again in the next post: completing it once
	// used to dismiss it for good, and Enter then sent "@bo".
	h.tt.Type("@bo")
	h.tt.Frame()
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	if got := *h.view.drafts[draftKey{"s1", "town", ""}]; got != "@bob " {
		t.Errorf("second completion: draft %q", got)
	}
}

// Someone typing shows for a few seconds, and then the view stops asking
// for frames.
func TestTypingIndicatorExpires(t *testing.T) {
	h := newHarness(t)
	h.srv.Store.Apply(store.Typing{Channel: "town", User: "bob", At: h.tt.Now()})
	h.tt.Frame()
	if !h.tt.HasText("bob is typing…") {
		t.Fatalf("%q", h.tt.Texts())
	}
	h.tt.Advance(store.TypingFor + time.Second)
	if h.tt.HasText("bob is typing…") {
		t.Error("still typing")
	}
	if h.tt.FramePending() {
		t.Error("idle view asks for frames")
	}
}

// Reply opens the thread in the RHS, and its composer replies to the root.
func TestThreadReply(t *testing.T) {
	h := newHarness(t)
	// The pointer over the first post shows its actions.
	r, _ := h.tt.Find("second")
	h.tt.Move(700, r.Y-30)
	if err := h.tt.Click("Reply"); err != nil {
		t.Fatalf("%v: %q", err, h.tt.Texts())
	}
	h.settle(func() bool { return h.tt.HasText("an earlier reply") })
	if h.srv.Thread != "p1" || !h.tt.HasText("Thread") {
		t.Fatalf("thread %q", h.srv.Thread)
	}
	h.snapshot("thread")
	// The reply box takes the focus as the thread opens.
	h.tt.Type("my reply")
	h.tt.Key(0, ui.KeyEnter)
	if b := <-h.posts; b["root_id"] != "p1" || b["message"] != "my reply" {
		t.Errorf("sent %v", b)
	}
	h.settle(func() bool { return len(h.srv.Store.Threads["p1"].Order) == 3 })
	if got := h.srv.Store.Timelines["town"].Order; len(got) != 2 {
		t.Errorf("the reply shows in the channel: %v", got)
	}
	if !h.tt.HasText("1 reply") {
		t.Errorf("no reply count under the root: %q", h.tt.Texts())
	}
	if err := h.tt.Click("Close thread"); err != nil {
		t.Fatal(err)
	}
	if h.srv.Thread != "" {
		t.Error("thread still open")
	}
}

// A still window asks for no frames.
func TestIdleAsksNoFrames(t *testing.T) {
	h := newHarness(t)
	h.tt.Advance(2 * time.Second)
	if h.tt.FramePending() {
		t.Error("idle view asks for frames")
	}
}

// A click anywhere on a post opens its thread, as the webapp's "Click to
// open threads"; a click on its reactions does not.
func TestClickPostOpensThread(t *testing.T) {
	h := newHarness(t)
	h.srv.Store.Apply(store.ReactionAdded{Post: "p2", Reaction: store.Reaction{UserID: "bob", EmojiName: "heart"}})
	h.tt.Frame()
	r, _ := h.tt.Find("❤")
	h.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	h.settle(func() bool { return true })
	if h.srv.Thread != "" {
		t.Fatalf("a reaction's click opened thread %q", h.srv.Thread)
	}
	h.srv.Store.Apply(store.PostCreated{Post: store.Post{ID: "lk", ChannelID: "town", UserID: "bob", CreateAt: time.Now().UnixMilli(), Message: "read [the docs](https://example.com/docs)"}})
	h.tt.Frame()
	r, _ = h.tt.Find("the docs")
	h.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	h.settle(func() bool { return true })
	if h.srv.Thread != "" {
		t.Fatalf("a link's click opened thread %q", h.srv.Thread)
	}
	r, _ = h.tt.Find("hello team")
	h.tt.ClickAt(r.X+r.W-50, r.Y+r.H/2)
	h.settle(func() bool { return h.tt.HasText("an earlier reply") })
	if h.srv.Thread != "p1" {
		t.Fatalf("thread %q", h.srv.Thread)
	}
	h.srv.Thread = ""
	h.tt.Frame()
	if err := h.tt.Click("Post by bob"); err != nil {
		t.Fatal(err)
	}
	h.settle(func() bool { return h.srv.Thread == "p1" })

	// A click on the text itself opens the thread too, Markdown or plain.
	for _, c := range []struct {
		text string
		post store.PostID
	}{{"hello team", "p1"}, {"second", "p2"}} {
		h.srv.Thread = ""
		h.tt.Frame()
		r, _ = h.tt.Find(c.text)
		h.tt.ClickAt(r.X+5, r.Y+r.H/2)
		h.settle(func() bool { return h.srv.Thread == c.post })
	}

	// A drag over the text selects it and opens nothing.
	h.srv.Thread = ""
	h.tt.Frame()
	r, _ = h.tt.Find("second")
	h.tt.Press(r.X+2, r.Y+r.H/2)
	h.tt.Frame()
	h.tt.Move(r.X+20, r.Y+r.H/2)
	h.tt.Frame()
	h.tt.Move(r.X+r.W-2, r.Y+r.H/2)
	h.tt.Frame()
	h.tt.Release(r.X+r.W-2, r.Y+r.H/2)
	h.settle(func() bool { return true })
	if h.srv.Thread != "" {
		t.Errorf("a drag opened thread %q", h.srv.Thread)
	}
}

// A system post is headed by the system avatar and name, as the webapp's.
func TestSystemPostHeader(t *testing.T) {
	h := newHarness(t)
	h.srv.Store.Apply(store.PostCreated{Post: store.Post{ID: "sys", ChannelID: "town", UserID: "bob", Type: "system_join_channel",
		CreateAt: time.Now().UnixMilli(), Message: "bob joined the channel."}})
	h.tt.Frame()
	if !h.tt.HasText("System") || !h.tt.HasText("bob joined the channel.") {
		t.Fatalf("%q", h.tt.Texts())
	}
	h.snapshot("system-post")
}

// A link's preview and a webhook's attachment show under their posts.
func TestPreviewsShow(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UnixMilli()
	h.srv.Store.Apply(store.PostCreated{Post: store.Post{ID: "l", ChannelID: "town", UserID: "bob", CreateAt: now,
		Message: "see https://github.com/mattermost/mattermost",
		Embed: &store.Embed{URL: "https://github.com/mattermost/mattermost", SiteName: "GitHub", Title: "GitHub - mattermost/mattermost",
			Description: "Mattermost is an open source platform", Image: &store.Image{URL: "https://example.invalid/i.png", Width: 1280, Height: 640}}}})
	h.srv.Store.Apply(store.PostCreated{Post: store.Post{ID: "w", ChannelID: "town", UserID: "bob", CreateAt: now + 1,
		Props: store.PostProps{OverrideUsername: "ci-bot", FromBot: true},
		Attachments: []store.Attachment{{Color: "danger", Pretext: "CI says", Title: "Build 42", TitleLink: "https://ci.example.com/42", Text: "**main** broke",
			Fields: []store.AttachmentField{{Title: "Branch", Value: "main", Short: true}, {Title: "Took", Value: "93 s", Short: true}}, Footer: "ci.example.com"}}}})
	h.tt.Frame()
	for _, want := range []string{"GitHub", "GitHub - mattermost/mattermost", "Mattermost is an open source platform", "ci-bot", "CI says", "Build 42", "broke", "Branch", "93 s", "ci.example.com"} {
		if !h.tt.HasText(want) {
			t.Errorf("no %q in %q", want, h.tt.Texts())
		}
	}
	h.snapshot("previews")
}

// The dark theme, saved for a look when MM_FRAMES is set.
func TestDarkLook(t *testing.T) {
	if os.Getenv("MM_FRAMES") == "" {
		t.Skip("MM_FRAMES not set")
	}
	h := newHarness(t)
	h.tt.SetDark(true)
	h.tt.Frame()
	h.snapshot("dark")
}
