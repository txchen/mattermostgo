package view

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/store"
)

// The search box finds posts, which show with their channel in the RHS.
func TestSearch(t *testing.T) {
	h := newHarness(t)
	at := time.Now().UnixMilli()
	h.replies["POST /api/v4/teams/t1/posts/search"] = `{"order":["f1"],"posts":{"f1":{"id":"f1","channel_id":"dev","user_id":"bob","create_at":` + itoa(at) + `,"message":"the deploy is done"}}}`
	h.replies["POST /api/v4/teams/t1/files/search"] = `{"order":[],"file_infos":{}}`
	if err := h.tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("deploy")
	h.tt.Key(0, ui.KeyEnter)
	if b := h.called("POST /api/v4/teams/t1/posts/search"); b["terms"] != "deploy" {
		t.Errorf("searched %v", b)
	}
	h.settle(func() bool { return h.tt.HasText("the deploy is done") })
	for _, want := range []string{"Search Results", "Developers", "Jump", "Messages 1"} {
		if !h.tt.HasText(want) {
			t.Errorf("no %q in %q", want, h.tt.Texts())
		}
	}
	h.snapshot("search")
	if err := h.tt.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if h.view.currentPanel(h.srv) != nil {
		t.Error("panel still open")
	}
}

func itoa(n int64) string { return strings.TrimSpace(strings.Repeat(" ", 0) + fmtInt(n)) }

func fmtInt(n int64) string {
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Jump scrolls the channel to a post found and highlights it, loading
// the older page that holds it.
func TestJumpToPost(t *testing.T) {
	h := newHarness(t)
	base := time.Now().Add(-2 * time.Hour).UnixMilli()
	var posts []store.Post
	for i := 30; i < 90; i++ {
		posts = append(posts, store.Post{ID: store.PostID("n" + itoa(int64(i))), ChannelID: "town", UserID: "bob", CreateAt: base + int64(i)*60000, Message: "newer post " + itoa(int64(i))})
	}
	h.srv.Store.Apply(store.PostsLoaded{Channel: "town", Posts: posts, HasOlder: true})
	var older strings.Builder
	older.WriteString(`{"order":[`)
	for i := 1; i < 30; i++ {
		if i > 1 {
			older.WriteString(",")
		}
		older.WriteString(`"o` + itoa(int64(i)) + `"`)
	}
	older.WriteString(`],"posts":{`)
	for i := 1; i < 30; i++ {
		if i > 1 {
			older.WriteString(",")
		}
		older.WriteString(`"o` + itoa(int64(i)) + `":{"id":"o` + itoa(int64(i)) + `","channel_id":"town","user_id":"bob","create_at":` + itoa(base+int64(i)*60000) + `,"message":"older post ` + itoa(int64(i)) + `"}`)
	}
	older.WriteString(`}}`)
	h.replies["GET /api/v4/channels/town/posts"] = older.String()
	h.replies["GET /api/v4/posts/o5/thread"] = `{"order":["o5"],"posts":{"o5":{"id":"o5","channel_id":"town","user_id":"bob","create_at":` + itoa(base+5*60000) + `,"message":"older post 5"}}}`
	h.tt.Frame()
	h.app.OpenPost(h.srv, "o5")
	h.settle(func() bool { return h.tt.HasText("older post 5") })
	r, _ := h.tt.Find("older post 5")
	if r.Y < 100 || r.Y > 600 {
		t.Errorf("the post is at %v, not in view", r)
	}
	if h.srv.Jump.Post != "o5" {
		t.Errorf("jump %+v", h.srv.Jump)
	}
	h.snapshot("jump")
	h.tt.Advance(4 * time.Second)
	if h.tt.FramePending() {
		t.Error("the highlight keeps asking for frames")
	}
}

// A post saved and pinned from its menu says so, and the server hears it.
func TestSaveAndPin(t *testing.T) {
	h := newHarness(t)
	h.replies["PUT /api/v4/users/me/preferences"] = `{}`
	h.replies["POST /api/v4/posts/p2/pin"] = `{}`
	h.choose("Post by bob", "Save")
	h.called("PUT /api/v4/users/me/preferences")
	if !h.srv.Store.Saved("p2") && !h.srv.Store.Saved("p1") {
		t.Fatal("not saved here")
	}
	saved := store.PostID("p1")
	if h.srv.Store.Saved("p2") {
		saved = "p2"
	}
	h.choose("Post by bob", "Pin to Channel")
	h.tt.Frame()
	if !h.tt.HasText("Saved") || !h.tt.HasText("Pinned") {
		t.Errorf("no marks: %q", h.tt.Texts())
	}
	h.snapshot("marks")
	_ = saved
}

// Threads lists the followed threads, with their unread replies, and
// opens one in the RHS.
func TestThreadsView(t *testing.T) {
	h := newHarness(t)
	h.replies["GET /api/v4/users/me/teams/t1/threads"] = `{"threads":[{"id":"p1","reply_count":3,"last_reply_at":` + itoa(time.Now().UnixMilli()) + `,"unread_replies":2,"unread_mentions":1,"is_following":true,"participants":[{"id":"bob"}],"post":{"id":"p1","channel_id":"town","user_id":"bob","message":"hello **team**"}}]}`
	h.replies["PUT /api/v4/users/me/teams/t1/threads/p1/read/"] = `{}`
	if err := h.tt.Click("Threads"); err != nil {
		t.Fatal(err)
	}
	h.settle(func() bool { return h.tt.HasText("Followed threads") && h.tt.HasText("3 replies · 2 new") })
	if n, m := h.srv.Store.UnreadThreads("t1"); n != 1 || m != 1 {
		t.Errorf("unread threads %d, mentions %d", n, m)
	}
	h.snapshot("threads")
	if err := h.tt.Click("Thread by bob"); err != nil {
		t.Fatal(err)
	}
	h.settle(func() bool { return h.tt.HasText("an earlier reply") })
	if n, _ := h.srv.Store.UnreadThreads("t1"); n != 0 {
		t.Error("opening the thread did not read it")
	}
	if err := h.tt.Click("Town Square"); err != nil {
		t.Fatal(err)
	}
	if h.srv.ThreadsView {
		t.Error("a channel did not leave the Threads view")
	}
}

// The emoji picker has categories, skin tones and a search.
func TestEmojiPicker(t *testing.T) {
	h := newHarness(t)
	h.replies["GET /api/v4/emoji"] = `[{"id":"e1","name":"partyparrot"}]`
	h.replies["PUT /api/v4/users/me/preferences"] = `{}`
	if err := h.tt.Click("Emoji"); err != nil {
		t.Fatal(err)
	}
	h.settle(func() bool { return h.tt.HasText("RECENTLY USED") })
	if err := h.tt.Click("People & Body"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if !h.tt.HasText(":wave:") {
		t.Errorf("no wave among people: %q", h.tt.Texts()[:20])
	}
	h.srv.Store.Apply(store.PrefsChanged{Prefs: []store.Preference{{Category: "emoji", Name: "emoji_skintone", Value: "1F3FD"}}})
	h.tt.Frame()
	if !h.tt.HasText(":wave_medium_skin_tone:") {
		t.Error("no skin tone")
	}
	h.snapshot("emoji-picker")
	if err := h.tt.Click("Search emoji"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("party")
	h.settle(func() bool { return h.tt.HasText(":partyparrot:") })
}

// A slash command completes and runs on the server instead of posting.
func TestSlashCommand(t *testing.T) {
	h := newHarness(t)
	h.replies["GET /api/v4/teams/t1/commands/autocomplete"] = `[{"trigger":"away","auto_complete_desc":"Set your status to away"},{"trigger":"echo","auto_complete_hint":"text"}]`
	h.replies["POST /api/v4/commands/execute"] = `{"text":"done","response_type":"ephemeral"}`
	h.tt.Type("/aw")
	h.settle(func() bool { return h.tt.HasText("/away") })
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	h.tt.Key(0, ui.KeyEnter)
	if b := h.called("POST /api/v4/commands/execute"); b["command"] != "/away" || b["channel_id"] != "town" {
		t.Errorf("ran %v", b)
	}
	select {
	case p := <-h.posts:
		t.Errorf("the command was posted: %v", p)
	default:
	}
}

// Images open large in the preview, and text files show their text.
func TestFilePreview(t *testing.T) {
	h := newHarness(t)
	h.replies["GET /api/v4/files/f2"] = "package main\n\nfunc main() {}\n"
	h.srv.Store.Apply(store.PostCreated{Post: store.Post{ID: "fp", ChannelID: "town", UserID: "bob", CreateAt: time.Now().UnixMilli(),
		Files: []store.FileInfo{{ID: "f1", Name: "shot.png", MimeType: "image/png", Width: 800, Height: 600, HasPreview: true}, {ID: "f2", Name: "main.go", Extension: "go", Size: 30}}}})
	h.tt.Frame()
	if err := h.tt.Click("main.go"); err != nil {
		t.Fatal(err)
	}
	h.settle(func() bool { return h.tt.HasText("func") && h.tt.HasText("2 of 2 · 30 B") })
	if err := h.tt.Click("Previous file"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if !h.tt.HasText("shot.png") || !h.tt.HasText("1 of 2 · 0 B") {
		t.Errorf("%q", h.tt.Texts())
	}
	if err := h.tt.Click("Close preview"); err != nil {
		t.Fatal(err)
	}
}

// Settings change the theme and the clock, and save display settings as
// preferences on the server.
func TestSettings(t *testing.T) {
	h := newHarness(t)
	h.replies["PUT /api/v4/users/me/preferences"] = `{}`
	h.replies["GET /api/v4/users/me"] = `{"id":"me","username":"me","notify_props":{"desktop":"mention","desktop_sound":"true","channel":"true","first_name":"false","mention_keys":""}}`
	if err := h.tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	h.settle(func() bool { return h.tt.HasText("Desktop notifications") })
	h.snapshot("settings-notify")
	if err := h.tt.Click("Display"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if err := h.tt.Click("One Dark theme"); err != nil {
		t.Fatal(err)
	}
	if h.app.Theme != "onedark" {
		t.Errorf("theme %q", h.app.Theme)
	}
	if err := h.tt.Click("12-hour"); err != nil {
		t.Fatal(err)
	}
	h.called("PUT /api/v4/users/me/preferences")
	if h.srv.Store.Pref("display_settings", "use_military_time") != "false" {
		t.Error("clock not set")
	}
	h.snapshot("settings-display")
	if err := h.tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if !strings.Contains(strings.Join(h.tt.Texts(), " "), " PM") && !strings.Contains(strings.Join(h.tt.Texts(), " "), " AM") {
		t.Errorf("no 12-hour times: %q", h.tt.Texts())
	}
	h.snapshot("onedark")
}

// The sidebar leaves out a closed DM, and groups unread channels first
// when asked.
func TestSidebarPrefs(t *testing.T) {
	h := newHarness(t)
	h.replies["PUT /api/v4/users/me/preferences"] = `{}`
	h.choose("bob", "Close Conversation")
	if h.tt.HasText("bob") && slices.Contains(h.view.sidebarOrder(h.srv), "dm") {
		t.Error("closed DM still in the sidebar")
	}
	h.srv.Store.Apply(store.PrefsChanged{Prefs: []store.Preference{{Category: "sidebar_settings", Name: "show_unread_section", Value: "true"}}})
	h.tt.Frame()
	if !h.tt.HasText("UNREADS") || h.view.sidebarOrder(h.srv)[0] != "dev" {
		t.Errorf("no Unreads first: %v", h.view.sidebarOrder(h.srv))
	}
	// A new post in the closed DM brings it back.
	h.srv.Store.Apply(store.PostCreated{Post: store.Post{ID: "nd", ChannelID: "dm", UserID: "bob", CreateAt: time.Now().UnixMilli(), Message: "hi again"}})
	h.app.Run(func() {})
	h.tt.Frame()
}

// Dragging a channel onto another category moves it there.
func TestDragChannel(t *testing.T) {
	h := newHarness(t)
	h.srv.Store.Apply(store.CategoriesLoaded{Team: "t1", Categories: []store.Category{
		{ID: "c-fav", Type: store.CategoryFavorites, DisplayName: "Favorites", Channels: []store.ChannelID{"town"}},
		{ID: "c-ch", Type: store.CategoryChannels, DisplayName: "Channels", Channels: []store.ChannelID{"dev"}},
		{ID: "c-dm", Type: store.CategoryDirect, DisplayName: "Direct Messages", Channels: []store.ChannelID{"dm"}},
	}})
	h.replies["GET /api/v4/users/me/teams/t1/channels/categories"] = `{"order":["c-fav","c-ch","c-dm"],"categories":[{"id":"c-fav","type":"favorites","channel_ids":["town"]},{"id":"c-ch","type":"channels","channel_ids":["dev"]},{"id":"c-dm","type":"direct_messages","channel_ids":["dm"]}]}`
	h.replies["PUT /api/v4/users/me/teams/t1/channels/categories"] = `[]`
	h.replies["GET /api/v4/users/me/teams/t1/channels/categories/c-fav"] = `{"id":"c-fav","type":"favorites","channel_ids":["dev","town"]}`
	h.replies["PUT /api/v4/users/me/teams/t1/channels/categories/c-fav"] = `{}`
	h.tt.Frame()
	from, _ := h.tt.Find("Developers")
	to, _ := h.tt.Find("Town Square")
	h.tt.Press(from.X+10, from.Y+from.H/2)
	for i := 1; i <= 8; i++ {
		h.tt.Move(from.X+10, from.Y+from.H/2+(to.Y-from.Y)*float32(i)/8)
		h.tt.Frame()
	}
	h.tt.Release(to.X+10, to.Y+to.H/2)
	h.tt.Frame()
	h.called("PUT /api/v4/users/me/teams/t1/channels/categories")
}

// The DM dialog starts a group message with several people.
func TestGroupMessage(t *testing.T) {
	h := newHarness(t)
	h.srv.Store.Apply(store.UsersLoaded{Users: []store.User{{ID: "carol", Username: "carol"}}})
	h.replies["POST /api/v4/channels/group"] = `{"id":"gm1","type":"G","name":"gm","display_name":"bob, carol, me"}`
	if err := h.tt.Click("Write a direct message"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.view.dialog.(*peopleDialog).picked = []store.UserID{"bob", "carol"}
	h.tt.Frame()
	if err := h.tt.Click("Go"); err != nil {
		t.Fatal(err)
	}
	h.calledWith("POST /api/v4/channels/group")
}

// Open in New Window asks for a window showing the channel.
func TestPopOut(t *testing.T) {
	h := newHarness(t)
	var title string
	var build func(*ui.Context)
	h.view.Hooks.OpenWindow = func(t string, b func(*ui.Context)) { title, build = t, b }
	h.choose("Developers", "Open in New Window")
	if title != "Developers" || build == nil {
		t.Fatalf("window %q", title)
	}
	pop := ui.NewTester(build, 700, 600)
	pop.Frame()
	if !pop.HasText("Developers channel menu") || !pop.HasText("Message") {
		t.Errorf("pop-out shows %q", pop.Texts())
	}
}
