package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

func login(t *testing.T, base, user string) (*mm.Client, store.User) {
	t.Helper()
	u, _ := url.Parse(base)
	c, me, err := mm.Login(context.Background(), http.DefaultClient, u, user, "Password1!", "")
	if err != nil {
		t.Fatal(err)
	}
	return c, me
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// categoryOf returns the one category listing a channel; a channel in
// none or in two returns no category.
func categoryOf(cats []store.Category, ch store.ChannelID) store.Category {
	var in []store.Category
	for _, c := range cats {
		if slices.Contains(c.Channels, ch) {
			in = append(in, c)
		}
	}
	if len(in) != 1 {
		return store.Category{DisplayName: fmt.Sprint(len(in), " categories")}
	}
	return in[0]
}

// The channel management calls do on the server what the webapp's do.
func TestParityChannelManagement(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	c, me := login(t, base, "alice")
	_, bob := login(t, base, "bob")
	ld := must(c.Load(ctx))
	team := ld.Teams[0].ID
	for _, tm := range ld.Teams {
		if tm.Name == "eng" {
			team = tm.ID
		}
	}
	name := fmt.Sprintf("parity-%d", time.Now().UnixNano())
	ch := must(c.CreateChannel(ctx, mm.NewChannel{TeamID: team, Name: name, DisplayName: "Parity", Type: store.Open, Purpose: "checks"}))
	if ch.DisplayName != "Parity" || ch.Purpose != "checks" {
		t.Errorf("created %+v", ch)
	}

	header, rename := "the header", "Parity Renamed"
	ch = must(c.PatchChannel(ctx, ch.ID, mm.ChannelPatch{Header: &header, DisplayName: &rename}))
	if ch.Header != header || ch.DisplayName != rename || ch.Name != name {
		t.Errorf("patched %+v", ch)
	}

	if err := c.AddMembers(ctx, ch.ID, []store.UserID{bob.ID}); err != nil {
		t.Fatal(err)
	}
	members := must(c.Members(ctx, ch.ID))
	var ids []store.UserID
	for _, m := range members {
		ids = append(ids, m.User)
		if m.User == me.ID && !m.Admin {
			t.Error("the creator is no channel admin")
		}
	}
	if !slices.Contains(ids, bob.ID) {
		t.Errorf("bob not added: %v", ids)
	}
	if err := c.RemoveMember(ctx, ch.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if n := must(c.MemberCount(ctx, ch.ID)); n != 1 {
		t.Errorf("%d members after removing bob", n)
	}

	if err := c.SetMarkUnread(ctx, me.ID, ch.ID, "mention"); err != nil {
		t.Fatal(err)
	}
	if m := must(c.Member(ctx, ch.ID)); !m.Muted() {
		t.Errorf("not muted: %+v", m)
	}
	if err := c.SetMarkUnread(ctx, me.ID, ch.ID, "all"); err != nil {
		t.Fatal(err)
	}

	post := must(c.CreatePost(ctx, mm.NewPost{ChannelID: ch.ID, Message: "to mark unread"}))
	if err := c.ViewChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
	if last := must(c.LastPost(ctx, ch.ID)); last != post.ID {
		t.Errorf("last post %q, want %q", last, post.ID)
	}
	if err := c.MarkUnread(ctx, me.ID, post.ID); err != nil {
		t.Fatal(err)
	}
	got := must(c.Channel(ctx, ch.ID))
	if m := must(c.Member(ctx, ch.ID)); m.MsgCount >= got.TotalMsgCount {
		t.Errorf("not unread: member %d of %d", m.MsgCount, got.TotalMsgCount)
	}

	cats := must(c.Categories(ctx, team))
	var fav, chans store.Category
	for _, cat := range cats {
		switch cat.Type {
		case store.CategoryFavorites:
			fav = cat
		case store.CategoryChannels:
			chans = cat
		}
	}
	if err := c.MoveToCategory(ctx, team, ch.ID, fav.ID); err != nil {
		t.Fatal(err)
	}
	cats = must(c.Categories(ctx, team))
	if got := categoryOf(cats, ch.ID); got.ID != fav.ID {
		t.Errorf("favorited channel is in %q", got.DisplayName)
	}
	catName := "Parity " + name[len(name)-6:]
	catID := must(c.CreateCategory(ctx, me.ID, team, catName))
	if err := c.MoveToCategory(ctx, team, ch.ID, catID); err != nil {
		t.Fatal(err)
	}
	cats = must(c.Categories(ctx, team))
	custom := categoryOf(cats, ch.ID)
	if custom.DisplayName != catName || custom.Type != store.CategoryCustom {
		t.Fatalf("new category holds it? %+v", custom)
	}
	if err := c.RenameCategory(ctx, team, custom.ID, catName+"!"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteCategory(ctx, team, custom.ID); err != nil {
		t.Fatal(err)
	}
	cats = must(c.Categories(ctx, team))
	if got := categoryOf(cats, ch.ID); got.ID != chans.ID {
		t.Errorf("after deleting its category the channel is in %q", got.DisplayName)
	}

	// Members may not convert a channel by default; an admin may.
	if _, err := c.MakePrivate(ctx, ch.ID); err == nil {
		t.Error("alice converted the channel")
	}
	admin, _ := login(t, base, "admin")
	ch = must(admin.MakePrivate(ctx, ch.ID))
	if ch.Type != store.Private {
		t.Errorf("type %q", ch.Type)
	}
	publics := must(c.PublicChannels(ctx, team))
	if !slices.ContainsFunc(publics, func(p store.Channel) bool { return p.Name == "town-square" }) {
		t.Error("no town square among public channels")
	}
	if err := c.ArchiveChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
	if got := must(c.Channel(ctx, ch.ID)); !got.Deleted {
		t.Error("not archived")
	}
}

func (c *client) loginAs(base, user string) *app.Server {
	c.a.AddServer("Local", base, func(err error) {
		if err != nil {
			c.t.Fatal(err)
		}
	})
	c.until("added", func() bool { return len(c.a.Servers) == 1 })
	c.a.Login(c.a.Current, user, "Password1!", "")
	c.until("ready", func() bool { return c.a.Current.Ready && c.a.Current.Channel != "" })
	s := c.a.Current
	c.until("ws online", func() bool { _, ok := s.Store.Conn.(store.ConnOnline); return ok })
	return s
}

func (c *client) choose(target string, path ...string) {
	c.t.Helper()
	open := c.tt.RightClick
	if strings.HasSuffix(target, " menu") {
		open = c.tt.Click
	}
	if err := open(target); err != nil {
		c.t.Fatalf("open menu of %q: %v", target, err)
	}
	if err := c.tt.ChooseMenuItem(path...); err != nil {
		c.t.Fatalf("menu %v: %v in %q", path, err, c.tt.Menu())
	}
	c.tt.Frame()
}

// Managing channels through the window, checked on the server.
func TestParityUIChannels(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	apiAlice, _ := login(t, base, "alice")
	c := newClient(t)
	s := c.loginAs(base, "alice")
	dev := channelByName(s, "dev")
	if c.a.IsFavorite(s, dev) {
		c.choose("Developers", "Unfavorite")
		c.until("dev not favorite", func() bool { return !c.a.IsFavorite(s, dev) })
	}

	c.choose("Developers", "Favorite")
	c.until("dev in favorites", func() bool { return c.a.IsFavorite(s, dev) && c.has("FAVORITES")() })
	if cat := categoryOf(must(apiAlice.Categories(ctx, s.Team)), dev); cat.Type != store.CategoryFavorites {
		t.Errorf("on the server dev is in %q", cat.DisplayName)
	}
	c.choose("Developers", "Unfavorite")
	c.until("dev back in channels", func() bool { return !c.a.IsFavorite(s, dev) })
	if cat := categoryOf(must(apiAlice.Categories(ctx, s.Team)), dev); cat.Type != store.CategoryChannels {
		t.Errorf("after unfavoriting, on the server dev is in %q", cat.DisplayName)
	}

	c.choose("Developers", "Mute Channel")
	muted := func() bool { m := must(apiAlice.Member(ctx, dev)); return m.Muted() }
	c.until("muted on the server", muted)
	c.choose("Developers", "Unmute Channel")
	c.until("unmuted on the server", func() bool { return !muted() })

	name := fmt.Sprintf("UI Parity %d", time.Now().UnixNano()%1000000)
	c.choose("Team menu", "Create New Channel")
	c.tt.Type(name)
	c.click("Create channel")
	c.until("new channel open", func() bool { ch := s.Store.Channels[s.Channel]; return ch != nil && ch.DisplayName == name })
	id := s.Channel

	c.choose(name+" channel menu", "Edit Channel Purpose")
	c.tt.Type("made from the window")
	c.click("Save")
	c.until("purpose shown", c.has("made from the window"))
	if got := must(apiAlice.Channel(ctx, id)); got.Purpose != "made from the window" {
		t.Errorf("server purpose %q", got.Purpose)
	}

	c.choose(name+" channel menu", "Add Members")
	c.tt.Type("bob")
	c.until("bob found", c.has("Bob Test"))
	c.click("bob")
	c.until("bob picked", c.has("Remove bob"))
	c.click("Add")
	c.until("two members", func() bool { return s.Store.Channels[id].MemberCount == 2 })

	c.choose(name+" channel menu", "Archive Channel")
	c.click("Archive")
	c.until("archived", func() bool { return s.Store.Channels[id] == nil })
	if got := must(apiAlice.Channel(ctx, id)); !got.Deleted {
		t.Error("not archived on the server")
	}
}

// The account menu sets the status, the custom status and the profile
// on the server.
func TestParityUIAccount(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	apiCarol, carol := login(t, base, "carol")
	c := newClient(t)
	s := c.loginAs(base, "carol")
	status := func() store.Status { return must(apiCarol.Statuses(ctx, []store.UserID{carol.ID}))[carol.ID] }

	c.choose("Account menu", "Away")
	c.until("away on the server", func() bool { return status() == store.Away })
	c.choose("Account menu", "Do not disturb", "30 mins")
	c.until("dnd on the server", func() bool { return status() == store.DND })
	c.choose("Account menu", "Online")
	c.until("online on the server", func() bool { return status() == store.Online })

	c.choose("Account menu", "Set custom status")
	c.click("Working from home")
	c.tt.Frame()
	c.click("Set Status")
	c.until("custom status on the server", func() bool {
		me := must(apiCarol.Me(ctx))
		return me.CustomStatus != nil && me.CustomStatus.Text == "Working from home" && me.CustomStatus.Emoji == "house"
	})
	c.choose("Account menu", "Clear custom status")
	c.until("custom status cleared", func() bool { return must(apiCarol.Me(ctx)).CustomStatus == nil })

	c.choose("Account menu", "Profile")
	c.tt.Key(0, ui.KeyTab)
	c.tt.Key(0, ui.KeyTab)
	c.tt.Type("cc")
	c.click("Save")
	c.until("nickname on the server", func() bool { return must(apiCarol.Me(ctx)).Nickname == "cc" })
	if s.Me().Nickname != "cc" {
		t.Errorf("nickname here %q", s.Me().Nickname)
	}
	if _, err := apiCarol.PatchMe(ctx, mm.UserPatch{FirstName: carol.FirstName, LastName: carol.LastName}); err != nil {
		t.Fatal(err)
	}
}

// An incoming webhook made in the window posts an attachment, and a link
// shows its preview, as they arrive over the WebSocket.
func TestParityUIWebhookAndPreviews(t *testing.T) {
	base := serverURL(t)
	c := newClient(t)
	s := c.loginAs(base, "admin")
	town := channelByName(s, "town-square")
	c.a.OpenChannel(s, town)
	c.choose("Team menu", "Integrations")
	c.until("integrations", c.has("Add Incoming Webhook"))
	c.click("Add Incoming Webhook")
	c.tt.Frame()
	title := fmt.Sprintf("UI hook %d", time.Now().UnixNano()%1000000)
	c.tt.Type(title)
	c.click("Save")
	c.until("hook URL", c.has(base+"/hooks/"))
	var hookURL string
	for _, text := range c.tt.Texts() {
		if strings.HasPrefix(text, base+"/hooks/") {
			hookURL = text
		}
	}
	c.click("Done")
	adminAPI, _ := login(t, base, "admin")
	for _, h := range must(adminAPI.IncomingHooks(context.Background(), s.Team)) {
		if h.DisplayName == title {
			town = store.ChannelID(h.ChannelID)
		}
	}
	c.a.OpenChannel(s, town)

	body := `{"text":"deploy report","attachments":[{"color":"good","title":"Deploy 7 done","text":"all **green**","fields":[{"title":"Env","value":"prod","short":true}]}]}`
	res, err := http.Post(hookURL, "application/json", strings.NewReader(body))
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("posting to the hook: %v %v", res, err)
	}
	c.until("attachment", func() bool { return c.has("Deploy 7 done")() && c.has("prod")() })

	api(t, base, "bob", "POST", "/posts", map[string]any{"channel_id": town, "message": "look https://github.com/mattermost/mattermost"})
	c.until("link preview", c.has("GitHub - mattermost/mattermost"))
	settled := time.Now().Add(2 * time.Second)
	c.until("preview image", func() bool { return time.Now().After(settled) })
}

// Integrations as an admin makes them: webhooks, commands, bots and tokens.
func TestParityIntegrations(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	c, _ := login(t, base, "admin")
	ld := must(c.Load(ctx))
	var team store.TeamID
	var town store.ChannelID
	for _, ch := range ld.Channels {
		if ch.Name == "town-square" && team == "" {
			team, town = ch.TeamID, ch.ID
		}
	}
	in := must(c.CreateIncomingHook(ctx, mm.IncomingHook{ChannelID: string(town), DisplayName: "parity hook", Description: "e2e"}))
	if !slices.ContainsFunc(must(c.IncomingHooks(ctx, team)), func(h mm.IncomingHook) bool { return h.ID == in.ID }) {
		t.Error("incoming hook not listed")
	}
	res, err := http.Post(c.HookURL(in.ID), "application/json", strings.NewReader(`{"text":"from the parity hook","attachments":[{"title":"Hook title","color":"good","text":"hook text"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("posting to the hook: %s", res.Status)
	}
	if err := c.DeleteIncomingHook(ctx, in.ID); err != nil {
		t.Fatal(err)
	}

	out := must(c.CreateOutgoingHook(ctx, mm.OutgoingHook{TeamID: string(team), ChannelID: string(town), DisplayName: "parity out",
		TriggerWords: []string{"paritytrigger"}, CallbackURLs: []string{"http://localhost:9/cb"}, ContentType: "application/json"}))
	if out.Token == "" {
		t.Error("outgoing hook has no token")
	}
	if err := c.DeleteOutgoingHook(ctx, out.ID); err != nil {
		t.Fatal(err)
	}

	trigger := fmt.Sprintf("parity%d", time.Now().Unix()%100000)
	cmd := must(c.CreateCommand(ctx, mm.Command{TeamID: string(team), Trigger: trigger, Method: "P", URL: "http://localhost:9/cmd", DisplayName: "Parity"}))
	if !slices.ContainsFunc(must(c.Commands(ctx, team)), func(x mm.Command) bool { return x.ID == cmd.ID }) {
		t.Error("command not listed")
	}
	if err := c.DeleteCommand(ctx, cmd.ID); err != nil {
		t.Fatal(err)
	}

	bot := must(c.CreateBot(ctx, mm.Bot{Username: trigger + "-bot", DisplayName: "Parity Bot", Description: "e2e"}))
	if err := c.AddToTeam(ctx, team, bot.UserID); err != nil {
		t.Fatal(err)
	}
	tok := must(c.CreateToken(ctx, bot.UserID, "e2e"))
	if tok.Token == "" {
		t.Fatal("no token")
	}
	u, _ := url.Parse(base)
	as := mm.New(u, tok.Token, http.DefaultClient)
	if me, err := as.Me(ctx); err != nil || me.Username != bot.Username {
		t.Errorf("the token is not the bot: %v %v", me, err)
	}
	if !slices.ContainsFunc(must(c.Bots(ctx)), func(b mm.Bot) bool { return b.UserID == bot.UserID }) {
		t.Error("bot not listed")
	}
	if b := must(c.SetBotEnabled(ctx, bot.UserID, false)); b.DeleteAt == 0 {
		t.Error("bot not disabled")
	}
	if b := must(c.SetBotEnabled(ctx, bot.UserID, true)); b.DeleteAt != 0 {
		t.Error("bot not enabled")
	}
}
