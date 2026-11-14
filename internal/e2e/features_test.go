package e2e

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// The third round through the window, checked on the server: search,
// saved and pinned posts, threads, slash commands, group messages,
// closing a DM, display settings and channel notifications.
func TestRound3UI(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	alice, me := login(t, base, "alice")
	bobC, bob := login(t, base, "bob")
	_, carol := login(t, base, "carol")
	c := newClient(t)
	s := c.loginAs(base, "alice")
	dev := channelByName(s, "dev")
	c.a.OpenChannel(s, dev)
	c.until("dev loaded", func() bool { tl := s.Store.Timelines[dev]; return tl != nil && !tl.Loading && len(tl.Order) > 0 })

	word := fmt.Sprintf("quokka%d", time.Now().UnixNano()%1000000)
	root := must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dev, Message: "about the " + word}))
	c.until("bob's post", c.has("about the "+word))

	// Search, through the top bar's box.
	time.Sleep(time.Second)
	c.click("Search")
	c.tt.Type(word)
	c.tt.Key(0, ui.KeyEnter)
	c.until("search result", func() bool { return c.has("Search Results")() && c.has("Messages 1")() })
	c.click("Close")

	// Save and pin from the post's menu.
	c.choosePost("about the "+word, "Save")
	c.until("saved on the server", func() bool {
		return slices.ContainsFunc(must(alice.Saved(ctx, me.ID)).Posts, func(p store.Post) bool { return p.ID == root.ID })
	})
	c.click("Saved messages")
	c.until("saved panel", func() bool { return c.has("Saved messages")() && c.has("about the "+word)() })
	c.choosePost("about the "+word, "Pin to Channel")
	c.until("pinned on the server", func() bool {
		return slices.ContainsFunc(must(alice.Pinned(ctx, dev)).Posts, func(p store.Post) bool { return p.ID == root.ID })
	})

	// A reply makes alice follow the thread; another one from bob is
	// unread in Threads.
	c.choosePost("about the "+word, "Reply")
	c.until("thread open", func() bool { return s.Thread == root.ID })
	c.tt.Type("I will look " + word)
	c.tt.Key(0, ui.KeyEnter)
	c.until("followed", func() bool { _, ok := s.Store.Followed[root.ID]; return ok })
	c.click("Close thread")
	must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dev, RootID: root.ID, Message: "thanks @alice"}))
	c.until("unread thread", func() bool { n, _ := s.Store.UnreadThreads(s.Team); return n > 0 })
	c.click("Threads")
	c.until("threads view", c.has("Followed threads"))
	c.click("Thread by bob")
	c.until("thread read", func() bool { n, _ := s.Store.UnreadThreads(s.Team); return n == 0 && s.Thread == root.ID })

	// A slash command runs on the server.
	c.a.OpenChannel(s, dev)
	c.until("dev open", func() bool { return !s.ThreadsView })
	c.tt.Key(0, ui.KeyEscape)
	c.a.Send(s, dev, "", "/away")
	c.until("away", func() bool { return must(alice.Statuses(ctx, []store.UserID{me.ID}))[me.ID] == store.Away })
	c.a.Send(s, dev, "", "/online")

	// A group message, and closing a DM.
	c.a.OpenGroup(s, []store.UserID{bob.ID, carol.ID})
	c.until("group open", func() bool { ch := s.Store.Channels[s.Channel]; return ch != nil && ch.Type == store.Group })
	var dm store.ChannelID
	for id, ch := range s.Store.Channels {
		if ch.Type == store.Direct && ch.DMPartner(me.ID) == bob.ID {
			dm = id
		}
	}
	if dm != "" {
		c.a.CloseConversation(s, dm)
		c.until("dm closed on the server", func() bool {
			for _, p := range must(alice.Preferences(ctx)) {
				if p.Category == "direct_channel_show" && p.Name == string(bob.ID) && p.Value == "false" {
					return true
				}
			}
			return false
		})
		must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dm, Message: "ping " + word}))
		c.until("dm back", func() bool { return !s.Store.Hidden(s.Store.Channels[dm]) })
	}

	// Display settings are preferences on the server.
	c.click("Settings")
	c.until("settings", c.has("Desktop notifications"))
	c.click("Display")
	c.tt.Frame()
	c.click("12-hour")
	c.until("clock on the server", func() bool {
		for _, p := range must(alice.Preferences(ctx)) {
			if p.Category == "display_settings" && p.Name == "use_military_time" {
				return p.Value == "false"
			}
		}
		return false
	})
	c.click("24-hour")
	c.click("Done")

	// The notification preferences of a channel.
	c.a.OpenChannel(s, dev)
	c.until("dev header", c.has("Developers channel menu"))
	c.choose("Developers channel menu", "Notification Preferences")
	c.until("channel notify", c.has("Ignore @channel, @here and @all"))
	c.click("Never")
	c.until("never on the server", func() bool { return must(alice.ChannelNotifyProps(ctx, dev))["desktop"] == "none" })
	c.click("Default")
	c.until("default again", func() bool { return must(alice.ChannelNotifyProps(ctx, dev))["desktop"] == "default" })
	c.click("Done")
	must(struct{}{}, alice.SetPinned(ctx, root.ID, false))
}

// A post sent while a channel opens shows in it: the page loading the
// channel used to replace the timeline and drop it.
func TestPostWhileOpening(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	bobC, _ := login(t, base, "bob")
	_, alice := login(t, base, "alice")
	// Two channels of its own, so that its posts push no others out of
	// the first page of a seeded channel.
	var team store.TeamID
	for _, tm := range must(bobC.Load(ctx)).Teams {
		if tm.Name == "eng" {
			team = tm.ID
		}
	}
	tag := fmt.Sprint(time.Now().UnixNano() % 1000000)
	var mine []string
	for _, n := range []string{"a", "b"} {
		ch := must(bobC.CreateChannel(ctx, mm.NewChannel{TeamID: team, Name: "opening-" + n + tag, DisplayName: "Opening " + n + tag, Type: store.Open}))
		if err := bobC.AddMembers(ctx, ch.ID, []store.UserID{alice.ID}); err != nil {
			t.Fatal(err)
		}
		mine = append(mine, ch.Name)
		id := ch.ID
		t.Cleanup(func() { bobC.ArchiveChannel(context.Background(), id) })
	}
	c := newClient(t)
	s := c.loginAs(base, "alice")
	c.until("own channels", func() bool { return channelByName(s, mine[0]) != "" && channelByName(s, mine[1]) != "" })
	for i, name := range []string{mine[0], mine[1], mine[0], mine[1], mine[0]} {
		ch := channelByName(s, name)
		other := channelByName(s, "dev")
		c.a.OpenChannel(s, other)
		// Drop the timeline, as one beyond the kept ones is, so that
		// opening loads the page again.
		c.a.ForgetTimeline(s, ch)
		c.a.OpenChannel(s, ch)
		p := must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: ch, Message: fmt.Sprintf("while opening %d %d", i, time.Now().UnixNano())}))
		c.until("post kept", func() bool {
			tl := s.Store.Timelines[ch]
			return tl != nil && !tl.Loading && slices.Contains(tl.Order, p.ID)
		})
	}
}

// A custom emoji made on the server draws in a message, and a permalink
// to a post shows the post it links to.
func TestRound5UI(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	admin, adminUser := login(t, base, "admin")
	bobC, _ := login(t, base, "bob")
	name := fmt.Sprintf("parrot%d", time.Now().UnixNano()%1000000)
	var pic bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 220, 255
	}
	png.Encode(&pic, img)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("image", name+".png")
	part.Write(pic.Bytes())
	w.WriteField("emoji", fmt.Sprintf(`{"creator_id":%q,"name":%q}`, adminUser.ID, name))
	w.Close()
	req, _ := http.NewRequest("POST", base+"/api/v4/emoji", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+admin.Token())
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode >= 300 {
		t.Fatalf("making the emoji: %v %v", res, err)
	}
	res.Body.Close()

	c := newClient(t)
	s := c.loginAs(base, "alice")
	dev := channelByName(s, "dev")
	c.a.OpenChannel(s, dev)
	c.until("dev loaded", func() bool { tl := s.Store.Timelines[dev]; return tl != nil && !tl.Loading && len(tl.Order) > 0 })
	quoted := must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dev, Message: "the quoted " + name}))
	must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dev, Message: "party :" + name + ": time"}))
	must(bobC.CreatePost(ctx, mm.NewPost{ChannelID: dev, Message: "see " + base + "/eng/pl/" + string(quoted.ID)}))
	label := "Open the linked message: the quoted " + name
	c.until("permalink preview", c.has(label))
	c.until("emoji drawn", func() bool { return c.has("party")() && !c.has(":"+name+":")() })
	c.click(label)
	c.until("jumped", func() bool { return s.Jump.Post == quoted.ID })
}

// choosePost chooses from the context menu of the post showing text.
func (c *client) choosePost(text string, path ...string) {
	c.t.Helper()
	r, ok := c.tt.Find(text)
	if !ok {
		c.t.Fatalf("no %q", text)
	}
	c.tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
	if err := c.tt.ChooseMenuItem(path...); err != nil {
		c.t.Fatalf("menu %v: %v in %q", path, err, c.tt.Menu())
	}
	c.tt.Frame()
}
