package e2e

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// The calls of the second round do on the server what the webapp's do.
func TestParity2API(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	c, me := login(t, base, "alice")
	bobC, bob := login(t, base, "bob")
	_, carol := login(t, base, "carol")
	admin, _ := login(t, base, "admin")
	ld := must(c.Load(ctx))
	if len(ld.Prefs) == 0 {
		t.Error("no preferences loaded")
	}
	var team store.TeamID
	for _, tm := range ld.Teams {
		if tm.Name == "eng" {
			team = tm.ID
		}
	}
	var dev store.ChannelID
	for _, ch := range ld.Channels {
		if ch.Name == "dev" && ch.TeamID == team {
			dev = ch.ID
		}
	}
	word := fmt.Sprintf("zebra%d", time.Now().UnixNano()%1000000)
	root := must(c.CreatePost(ctx, mm.NewPost{ChannelID: dev, Message: "searchable " + word}))
	must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dev, RootID: root.ID, Message: "a reply for @alice"}))

	var found []store.PostID
	for range 20 {
		res := must(c.Search(ctx, team, word))
		found = nil
		for _, p := range res.Posts {
			found = append(found, p.ID)
		}
		if len(found) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !slices.Contains(found, root.ID) {
		t.Errorf("search for %s found %v", word, found)
	}

	if err := c.SetPrefs(ctx, me.ID, []store.Preference{{Category: "flagged_post", Name: string(root.ID), Value: "true"}}); err != nil {
		t.Fatal(err)
	}
	saved := must(c.Saved(ctx, me.ID))
	if !slices.ContainsFunc(saved.Posts, func(p store.Post) bool { return p.ID == root.ID }) {
		t.Error("saved post not listed")
	}
	if err := c.DeletePrefs(ctx, me.ID, []store.Preference{{Category: "flagged_post", Name: string(root.ID), Value: "true"}}); err != nil {
		t.Fatal(err)
	}

	if err := c.SetPinned(ctx, root.ID, true); err != nil {
		t.Fatal(err)
	}
	pinned := must(c.Pinned(ctx, dev))
	if !slices.ContainsFunc(pinned.Posts, func(p store.Post) bool { return p.ID == root.ID && p.Pinned }) {
		t.Error("pinned post not listed as pinned")
	}
	if err := c.SetPinned(ctx, root.ID, false); err != nil {
		t.Fatal(err)
	}

	if err := c.Remind(ctx, me.ID, root.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	must(c.EditPost(ctx, root.ID, "searchable "+word+" edited"))
	if hist := must(c.EditHistory(ctx, root.ID)); len(hist) == 0 {
		t.Error("no edit history")
	}

	threads := must(c.Threads(ctx, me.ID, team))
	if !slices.ContainsFunc(threads.Threads, func(th store.ThreadState) bool { return th.Root == root.ID && th.Following }) {
		t.Errorf("the thread is not followed: %+v", threads.Threads)
	}
	if err := c.ReadThread(ctx, me.ID, team, root.ID, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetFollowing(ctx, me.ID, team, root.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := c.SetFollowing(ctx, me.ID, team, root.ID, true); err != nil {
		t.Fatal(err)
	}

	cmds := must(c.AutocompleteCommands(ctx, team))
	if !slices.ContainsFunc(cmds, func(x mm.AutocompleteCommand) bool { return x.Trigger == "away" }) {
		t.Errorf("no /away among %d commands", len(cmds))
	}
	if _, err := c.RunCommand(ctx, team, dev, "", "/online"); err != nil {
		t.Errorf("running /online: %v", err)
	}
	_ = must(c.CustomEmojis(ctx))

	gm := must(c.GroupChannel(ctx, []store.UserID{me.ID, bob.ID, carol.ID}))
	if gm.Type != store.Group {
		t.Errorf("group %+v", gm)
	}

	name := fmt.Sprintf("parity2-%d", time.Now().UnixNano()%1000000)
	ch := must(c.CreateChannel(ctx, mm.NewChannel{TeamID: team, Name: name, DisplayName: "Parity2", Type: store.Private}))
	if err := c.AddMembers(ctx, ch.ID, []store.UserID{bob.ID}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetChannelAdmin(ctx, ch.ID, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if ms := must(c.Members(ctx, ch.ID)); !slices.ContainsFunc(ms, func(m mm.MemberInfo) bool { return m.User == bob.ID && m.Admin }) {
		t.Error("bob is not channel admin")
	}
	if _, err := admin.MakePublic(ctx, ch.ID); err != nil {
		t.Errorf("making public: %v", err)
	}
	if b, err := c.AddBookmark(ctx, ch.ID, mm.Bookmark{DisplayName: "Docs", LinkURL: "https://example.com"}); err != nil {
		t.Logf("bookmarks: %v", err)
	} else if bs := must(c.Bookmarks(ctx, ch.ID)); len(bs) != 1 || bs[0].ID != b.ID {
		t.Errorf("bookmarks %+v", bs)
	}
	if err := c.SetChannelNotify(ctx, me.ID, ch.ID, map[string]string{"desktop": "none"}); err != nil {
		t.Fatal(err)
	}
	if p := must(c.ChannelNotifyProps(ctx, ch.ID)); p["desktop"] != "none" {
		t.Errorf("channel notify %v", p)
	}
	if err := c.ArchiveChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
	if arch := must(c.ArchivedChannels(ctx, team)); !slices.ContainsFunc(arch, func(x store.Channel) bool { return x.ID == ch.ID }) {
		t.Error("archived channel not listed")
	}
	if got := must(admin.RestoreChannel(ctx, ch.ID)); got.Deleted {
		t.Error("not restored")
	}
	must(struct{}{}, c.ArchiveChannel(ctx, ch.ID))

	old := must(c.NotifyProps(ctx))
	if _, err := c.SetNotify(ctx, map[string]string{"mention_keys": "parity,alice"}); err != nil {
		t.Fatal(err)
	}
	if p := must(c.NotifyProps(ctx)); p["mention_keys"] != "parity,alice" || p["desktop"] != old["desktop"] {
		t.Errorf("notify props %v", p)
	}
	must(c.SetNotify(ctx, map[string]string{"mention_keys": old["mention_keys"]}))

	if secret := must(c.GenerateMFA(ctx, me.ID)); secret.Secret == "" || len(secret.QRCode) < 100 {
		t.Errorf("MFA secret %q, QR %d bytes", secret.Secret, len(secret.QRCode))
	}
	if ss := must(c.Sessions(ctx, me.ID)); len(ss) == 0 {
		t.Error("no sessions")
	}
	if inv := must(admin.TeamInvite(ctx, team)); inv == "" {
		t.Error("no invite id")
	}

	cats := must(c.Categories(ctx, team))
	var chans store.Category
	var order []string
	for _, cat := range cats {
		order = append(order, cat.ID)
		if cat.Type == store.CategoryChannels {
			chans = cat
		}
	}
	if err := c.PatchCategory(ctx, team, chans.ID, map[string]any{"sorting": "recent", "collapsed": true}); err != nil {
		t.Fatal(err)
	}
	cats = must(c.Categories(ctx, team))
	for _, cat := range cats {
		if cat.ID == chans.ID && (cat.Sorting != "recent" || !cat.Collapsed) {
			t.Errorf("category %+v", cat)
		}
	}
	must(struct{}{}, c.PatchCategory(ctx, team, chans.ID, map[string]any{"sorting": "alpha", "collapsed": false}))
	if err := c.SetCategoryOrder(ctx, team, order); err != nil {
		t.Fatal(err)
	}
}
