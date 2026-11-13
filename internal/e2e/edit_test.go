package e2e

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/mm"
)

// Integrations edit in the window, and the server keeps the changes.
func TestEditIntegrations(t *testing.T) {
	base := serverURL(t)
	ctx := context.Background()
	admin, _ := login(t, base, "admin")
	c := newClient(t)
	s := c.loginAs(base, "admin")
	town := channelByName(s, "town-square")
	tag := fmt.Sprint(time.Now().UnixNano() % 1000000)
	in := must(admin.CreateIncomingHook(ctx, mm.IncomingHook{ChannelID: string(town), DisplayName: "hook " + tag}))
	out := must(admin.CreateOutgoingHook(ctx, mm.OutgoingHook{TeamID: string(s.Team), ChannelID: string(town), DisplayName: "out " + tag,
		TriggerWords: []string{"w" + tag}, CallbackURLs: []string{"http://localhost:9/a"}, ContentType: "application/json"}))
	cmd := must(admin.CreateCommand(ctx, mm.Command{TeamID: string(s.Team), Trigger: "e" + tag, Method: "P", URL: "http://localhost:9/c", DisplayName: "cmd " + tag}))
	bot := must(admin.CreateBot(ctx, mm.Bot{Username: "edit" + tag + "-bot", DisplayName: "Edit bot"}))

	// replace types over a field of the form, reached with Tab from the
	// first one, which has the focus.
	replace := func(tabs int, text string) {
		c.t.Helper()
		for range tabs {
			c.tt.Key(0, ui.KeyTab)
		}
		c.tt.Key(ui.Cmd, ui.KeyA)
		c.tt.Type(text)
		c.tt.Frame()
	}

	c.choose("Team menu", "Integrations")
	c.until("hooks", c.has("hook "+tag))
	c.click("Edit hook " + tag)
	c.tt.Frame()
	replace(0, "renamed "+tag)
	c.click("Save")
	c.until("hook renamed", func() bool {
		return slices.ContainsFunc(must(admin.IncomingHooks(ctx, s.Team)), func(h mm.IncomingHook) bool {
			return h.ID == in.ID && h.DisplayName == "renamed "+tag && h.ChannelID == string(town)
		})
	})

	c.click("Outgoing Webhooks")
	c.until("outgoing", c.has("out "+tag))
	c.click("Edit out " + tag)
	c.tt.Frame()
	replace(6, "http://localhost:9/b")
	c.click("Save")
	c.until("callback changed", func() bool {
		return slices.ContainsFunc(must(admin.OutgoingHooks(ctx, s.Team)), func(h mm.OutgoingHook) bool {
			return h.ID == out.ID && slices.Equal(h.CallbackURLs, []string{"http://localhost:9/b"}) && slices.Equal(h.TriggerWords, []string{"w" + tag})
		})
	})

	c.click("Slash Commands")
	c.until("commands", c.has("cmd "+tag))
	c.click("Edit /e" + tag)
	c.tt.Frame()
	replace(3, "http://localhost:9/d")
	c.click("Save")
	c.until("command changed", func() bool {
		return slices.ContainsFunc(must(admin.Commands(ctx, s.Team)), func(x mm.Command) bool { return x.ID == cmd.ID && x.URL == "http://localhost:9/d" && x.Method == "P" })
	})

	c.click("Bot Accounts")
	c.until("bots", c.has("@"+bot.Username))
	c.click("Edit " + bot.Username)
	c.tt.Frame()
	replace(2, "edited in the window")
	c.click("Save")
	c.until("bot changed", func() bool {
		return slices.ContainsFunc(must(admin.Bots(ctx)), func(b mm.Bot) bool { return b.UserID == bot.UserID && b.Description == "edited in the window" })
	})
}
