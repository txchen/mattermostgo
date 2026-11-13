// Screenshot draws the window headless with made-up data, for the README:
//
//	go run ./scripts/screenshot -out docs/screenshots
//
// It writes light.png (Denim) and dark.png (One Dark) at twice the size.
package main

import (
	"flag"
	"image/png"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/config"
	"github.com/txchen/mattermostgo/internal/media"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
	"github.com/txchen/mattermostgo/internal/view"
)

type noSecrets struct{}

func (noSecrets) Get(string) ([]byte, error) { return nil, nil }
func (noSecrets) Set(string, []byte) error   { return nil }
func (noSecrets) Delete(string) error        { return nil }

func main() {
	out := flag.String("out", "docs/screenshots", "directory to write the pictures to")
	flag.Parse()
	hs := httptest.NewServer(http.NotFoundHandler())
	defer hs.Close()
	u, _ := url.Parse(hs.URL)
	var queue []func()
	a := &app.App{Run: func(f func()) { queue = append(queue, f) }, HC: hs.Client(), Secrets: noSecrets{}, ConfigPath: filepath.Join(os.TempDir(), "mmgo-shot.json")}
	s := &app.Server{Config: config.Server{ID: "shot", Name: "Acme", URL: "https://chat.acme.dev"}, URL: u,
		Session: app.LoggedIn{Client: mm.New(u, "tok", hs.Client())}, Store: store.New(), Ready: true}
	a.Servers, a.Current = []*app.Server{s}, s
	v := view.New(a, media.New(a.Run, "", 1<<20))
	fill(s)
	for _, theme := range []struct{ id, file string }{{"denim", "light.png"}, {"onedark", "dark.png"}} {
		a.Theme = theme.id
		tt := ui.NewTester(v.Build, 1500, 920)
		tt.SetScale(2)
		for range 6 {
			for _, f := range queue {
				f()
			}
			queue = nil
			tt.Frame()
			time.Sleep(50 * time.Millisecond)
		}
		f, err := os.Create(filepath.Join(*out, theme.file))
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			log.Fatal(err)
		}
		f.Close()
	}
}

// fill gives the store a team, channels, people and a conversation.
func fill(s *app.Server) {
	st := s.Store
	day := time.Now()
	at := func(h, m int) int64 {
		y, mo, d := day.Date()
		return time.Date(y, mo, d, h, m, 0, 0, time.Local).UnixMilli()
	}
	ch := func(id, name, display string, typ store.ChannelType, total int64) store.Channel {
		return store.Channel{ID: store.ChannelID(id), TeamID: "acme", Type: typ, Name: name, DisplayName: display, TotalMsgCount: total, LastPostAt: at(9, 0)}
	}
	st.Apply(store.Loaded{
		Me:    store.User{ID: "me", Username: "sam", FirstName: "Sam", LastName: "Rivera"},
		Teams: []store.Team{{ID: "acme", Name: "acme", DisplayName: "Acme"}, {ID: "labs", Name: "labs", DisplayName: "Labs"}},
		Channels: []store.Channel{
			ch("eng", "engineering", "engineering", store.Open, 40),
			ch("design", "design", "design", store.Open, 12),
			ch("releases", "releases", "releases", store.Open, 9),
			ch("incidents", "incidents", "incidents", store.Open, 6),
			ch("town", "town-square", "Town Square", store.Open, 30),
			ch("lead", "leadership", "leadership", store.Private, 4),
			{ID: "dm-maya", Type: store.Direct, Name: "maya__me", TotalMsgCount: 3, LastPostAt: at(9, 40)},
			{ID: "dm-leo", Type: store.Direct, Name: "leo__me", TotalMsgCount: 2, LastPostAt: at(8, 15)},
			{ID: "dm-bot", Type: store.Direct, Name: "deploy-bot__me", TotalMsgCount: 1, LastPostAt: at(7, 0)},
		},
		Members: []store.ChannelMember{
			{ChannelID: "eng", MsgCount: 40}, {ChannelID: "design", MsgCount: 12}, {ChannelID: "releases", MsgCount: 7},
			{ChannelID: "incidents", MsgCount: 4, MentionCount: 2}, {ChannelID: "town", MsgCount: 30}, {ChannelID: "lead", MsgCount: 4},
			{ChannelID: "dm-maya", MsgCount: 2, MentionCount: 1}, {ChannelID: "dm-leo", MsgCount: 2}, {ChannelID: "dm-bot", MsgCount: 1},
		},
	})
	st.Apply(store.UsersLoaded{Users: []store.User{
		{ID: "maya", Username: "maya", FirstName: "Maya", LastName: "Chen", Position: "Staff Engineer"},
		{ID: "leo", Username: "leo", FirstName: "Leo", LastName: "Park"},
		{ID: "ines", Username: "ines", FirstName: "Inés", LastName: "Duarte", CustomStatus: &store.CustomStatus{Emoji: "rocket", Text: "Shipping 2.4"}},
		{ID: "deploy-bot", Username: "deploy-bot", IsBot: true},
	}})
	st.Apply(store.StatusesChanged{Statuses: map[store.UserID]store.Status{"me": store.Online, "maya": store.Online, "leo": store.Away, "ines": store.DND}})
	st.Apply(store.CategoriesLoaded{Team: "acme", Categories: []store.Category{
		{ID: "fav", Type: store.CategoryFavorites, DisplayName: "Favorites", Channels: []store.ChannelID{"eng", "releases"}},
		{ID: "chans", Type: store.CategoryChannels, DisplayName: "Channels", Channels: []store.ChannelID{"design", "incidents", "town", "lead"}},
		{ID: "dms", Type: store.CategoryDirect, DisplayName: "Direct Messages", Sorting: "recent", Channels: []store.ChannelID{"dm-maya", "dm-leo", "dm-bot"}},
	}})
	st.Apply(store.ThreadsLoaded{Team: "acme", Threads: []store.ThreadState{{Root: "p2", Team: "acme", ReplyCount: 4, UnreadReplies: 2, UnreadMentions: 1, Following: true}}})
	s.Team, s.Channel = "acme", "eng"
	post := func(id, user string, h, m int, msg string) store.Post {
		return store.Post{ID: store.PostID(id), ChannelID: "eng", UserID: store.UserID(user), CreateAt: at(h, m), Message: msg}
	}
	p1 := post("p1", "maya", 9, 2, "Morning! The **2.4 release branch** is cut. I moved the flaky search tests behind `-short`, so CI is green again.")
	p1.Reactions = []store.Reaction{{UserID: "leo", EmojiName: "tada"}, {UserID: "ines", EmojiName: "tada"}, {UserID: "me", EmojiName: "+1"}}
	p2 := post("p2", "leo", 9, 14, "Before we tag it, can someone look at the cache change? It cuts cold starts from 1.8 s to 420 ms on my machine:\n\n```go\nfunc (c *Cache) Get(key string) ([]byte, bool) {\n\tc.mu.RLock()\n\tdefer c.mu.RUnlock()\n\te, ok := c.items[key]\n\treturn e.data, ok && time.Now().Before(e.expires)\n}\n```")
	p2.ReplyCount, p2.LastReplyAt, p2.Participants = 4, at(9, 31), []store.UserID{"maya", "ines", "me"}
	p4 := post("p4", "ines", 9, 26, "Design review notes are up: https://acme.dev/design/2.4")
	p4.Embed = &store.Embed{URL: "https://acme.dev/design/2.4", SiteName: "Acme Design", Title: "2.4 review: onboarding and the new settings", Description: "Flows, open questions and the decisions we made on Tuesday."}
	p5 := post("p5", "deploy-bot", 9, 40, "")
	p5.Props.FromBot = true
	p5.Attachments = []store.Attachment{{Color: "good", Title: "Staging deploy finished: api 2.4.0-rc1", Text: "All 312 checks passed in 6 min 41 s.",
		Fields:  []store.AttachmentField{{Title: "Environment", Value: "staging", Short: true}, {Title: "Commit", Value: "`9f3c2e1`", Short: true}},
		Actions: []store.Action{{ID: "promote", Name: "Promote to production", Style: "primary"}, {ID: "rollback", Name: "Roll back"}}, Footer: "deploy-bot"}}
	st.Apply(store.PostsLoaded{Channel: "eng", Posts: []store.Post{p5, p4, p2, p1}})
	reply := func(id, user string, h, m int, msg string) store.Post {
		r := post(id, user, h, m, msg)
		r.RootID = "p2"
		return r
	}
	st.Apply(store.ThreadLoaded{Root: "p2", Posts: []store.Post{p2,
		reply("r1", "maya", 9, 18, "Nice. Does the read lock hold up under the import job? That one hammers `Get`."),
		reply("r2", "leo", 9, 21, "Ran it with 64 workers: p99 stays under 2 ms."),
		reply("r3", "ines", 9, 29, "Then let us ship it in 2.4 :rocket:"),
		reply("r4", "me", 9, 31, "Approved. I will tag the release after lunch."),
	}})
	s.Thread = "p2"
	st.Apply(store.ConnChanged{Conn: store.ConnOnline{}})
}
