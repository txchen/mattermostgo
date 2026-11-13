package e2e

import (
	"context"
	"fmt"
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
