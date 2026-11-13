package view

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/store"
)

// choose chooses from the menu of an element: its context menu, or with
// a menu button, the one it opens.
func (h *harness) choose(target string, path ...string) {
	h.t.Helper()
	open := h.tt.RightClick
	if strings.HasSuffix(target, " menu") {
		open = h.tt.Click
	}
	if err := open(target); err != nil {
		h.t.Fatalf("right click %q: %v", target, err)
	}
	if err := h.tt.ChooseMenuItem(path...); err != nil {
		h.t.Fatalf("menu %v: %v in %q", path, err, h.tt.Menu())
	}
	h.tt.Frame()
}

// A channel in the sidebar has the webapp's menu, on a right click and on
// the button its row shows under the pointer.
func TestSidebarChannelMenu(t *testing.T) {
	h := newHarness(t)
	if err := h.tt.RightClick("Developers"); err != nil {
		t.Fatal(err)
	}
	items := h.tt.Menu()
	for _, want := range []string{"Favorite", "Mute Channel", "Move to…", "Copy Link", "Add Members", "Leave Channel"} {
		if !slices.Contains(items, want) {
			t.Errorf("no %q in %q", want, items)
		}
	}
	h.tt.CloseMenu()

	h.choose("Developers", "Mute Channel")
	if !h.srv.Store.Members["dev"].Muted() {
		t.Error("not muted at once")
	}
	if b := h.called("PUT /api/v4/channels/dev/members/me/notify_props"); b["mark_unread"] != "mention" {
		t.Errorf("sent %v", b)
	}

	h.replies["POST /api/v4/users/me/teams/t1/channels/categories"] = `{"id":"c-work"}`
	h.replies["GET /api/v4/users/me/teams/t1/channels/categories"] = `{"order":["c-ch","c-work"],"categories":[{"id":"c-ch","type":"channels","channel_ids":["town","dev"]},{"id":"c-work","type":"custom","channel_ids":[]}]}`
	h.replies["PUT /api/v4/users/me/teams/t1/channels/categories"] = `[]`
	h.choose("Developers", "Move to…", "New Category…")
	if !h.tt.HasText("Create New Category") {
		t.Fatalf("no dialog: %q", h.tt.Texts())
	}
	h.tt.Type("Work")
	h.tt.Key(0, ui.KeyEnter)
	if b := h.called("POST /api/v4/users/me/teams/t1/channels/categories"); b["display_name"] != "Work" {
		t.Errorf("sent %v", b)
	}
	// The channel moves in and out of its old category in one update.
	moved := fmt.Sprint(h.calledWith("PUT /api/v4/users/me/teams/t1/channels/categories"))
	if !strings.Contains(moved, "channel_ids:[town]") || !strings.Contains(moved, "channel_ids:[dev]") {
		t.Errorf("moved with %s", moved)
	}
	if v := h.view.dialog; v != nil {
		t.Errorf("dialog still open: %T", v)
	}

	h.tt.Move(100, 0)
	r, _ := h.tt.Find("Developers")
	h.tt.Move(r.X+20, r.Y+r.H/2)
	h.tt.Frame()
	if !h.tt.HasText("Channel options for Developers") {
		t.Errorf("no options button under the pointer: %q", h.tt.Texts())
	}
}

// The channel name in the header opens the channel menu, which edits the
// header.
func TestHeaderChannelMenu(t *testing.T) {
	h := newHarness(t)
	h.replies["PUT /api/v4/channels/town/patch"] = `{"id":"town","team_id":"t1","type":"O","name":"town-square","display_name":"Town Square","header":"new header"}`
	h.choose("Town Square channel menu", "Edit Channel Header")
	if !h.tt.HasText("Edit Header for Town Square") {
		t.Fatalf("no dialog: %q", h.tt.Texts())
	}
	h.tt.Type("new header")
	if err := h.tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if b := h.called("PUT /api/v4/channels/town/patch"); b["header"] != "new header" || b["display_name"] != nil {
		t.Errorf("sent %v", b)
	}
	h.settle(func() bool { return h.srv.Store.Channels["town"].Header == "new header" })
	h.choose("Town Square channel menu", "View Members")
	if _, ok := h.view.dialog.(*membersDialog); !ok {
		t.Errorf("dialog %T", h.view.dialog)
	}
}

// The sidebar's + creates a channel, which then opens.
func TestCreateChannel(t *testing.T) {
	h := newHarness(t)
	h.replies["POST /api/v4/channels"] = `{"id":"newch","team_id":"t1","type":"P","name":"release-plans","display_name":"Release Plans"}`
	h.replies["GET /api/v4/channels/newch"] = h.replies["POST /api/v4/channels"]
	h.replies["GET /api/v4/channels/newch/members/me"] = `{"channel_id":"newch","msg_count_root":0}`
	h.replies["GET /api/v4/users/me/teams/t1/channels/categories"] = `{"order":["c-ch"],"categories":[{"id":"c-ch","type":"channels","display_name":"Channels","channel_ids":["newch","town","dev"]}]}`
	if err := h.tt.Click("New"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if err := h.tt.Click("Create new channel"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.tt.Type("Release Plans")
	if !h.tt.HasText("URL: 127.0.0.1") && !strings.Contains(strings.Join(h.tt.Texts(), "\n"), "/channels/release-plans") {
		t.Errorf("no URL shown: %q", h.tt.Texts())
	}
	if err := h.tt.Click("Private channel"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if err := h.tt.Click("Create channel"); err != nil {
		t.Fatal(err)
	}
	b := h.called("POST /api/v4/channels")
	if b["name"] != "release-plans" || b["display_name"] != "Release Plans" || b["type"] != "P" || b["team_id"] != "t1" {
		t.Errorf("sent %v", b)
	}
	h.settle(func() bool { return h.srv.Channel == "newch" })
	if h.view.dialog != nil {
		t.Errorf("dialog still open: %T", h.view.dialog)
	}
}

// The team menu opens the integrations, which make an incoming webhook
// and show its URL.
func TestIncomingWebhook(t *testing.T) {
	h := newHarness(t)
	h.replies["GET /api/v4/hooks/incoming"] = `[]`
	h.replies["POST /api/v4/hooks/incoming"] = `{"id":"hook1","channel_id":"dev","display_name":"CI"}`
	h.choose("Team menu", "Integrations")
	h.settle(func() bool { return h.tt.HasText("No incoming webhooks yet") })
	if err := h.tt.Click("Add Incoming Webhook"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.tt.Type("CI")
	h.view.dialog.(*integrations).adding.(*incomingForm).channel = "Developers"
	h.tt.Frame()
	if err := h.tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	b := h.called("POST /api/v4/hooks/incoming")
	if b["display_name"] != "CI" || b["channel_id"] != "dev" || b["channel_locked"] != true {
		t.Errorf("sent %v", b)
	}
	h.settle(func() bool { return h.tt.HasText(h.srv.URL.String() + "/hooks/hook1") })
	h.snapshot("integrations")
}

// The avatar in the top bar opens the account menu: status, custom
// status, profile and log out.
func TestAccountMenu(t *testing.T) {
	h := newHarness(t)
	if err := h.tt.Click("Account menu"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	items := h.tt.Menu()
	for _, want := range []string{"@me", "Set custom status", "Online", "Away", "Do not disturb", "Offline", "Profile", "Log out"} {
		if !slices.Contains(items, want) {
			t.Errorf("no %q in %q", want, items)
		}
	}
	h.snapshot("account-menu")
	h.tt.CloseMenu()

	h.replies["PUT /api/v4/users/me/status"] = `{}`
	h.choose("Account menu", "Away")
	if b := h.called("PUT /api/v4/users/me/status"); b["status"] != "away" || b["user_id"] != "me" {
		t.Errorf("sent %v", b)
	}
	if h.srv.Store.Statuses["me"] != store.Away {
		t.Error("status not shown at once")
	}
	h.mu.Lock()
	h.calls = nil
	h.mu.Unlock()
	h.choose("Account menu", "Do not disturb", "1 hour")
	b := h.called("PUT /api/v4/users/me/status")
	if end, _ := b["dnd_end_time"].(float64); b["status"] != "dnd" || end < float64(time.Now().Add(59*time.Minute).Unix()) {
		t.Errorf("sent %v", b)
	}

	h.replies["PUT /api/v4/users/me/status/custom"] = `{}`
	h.choose("Account menu", "Set custom status")
	if err := h.tt.Click("In a meeting"); err != nil {
		t.Fatalf("%v in %q", err, h.tt.Texts())
	}
	h.tt.Frame()
	if err := h.tt.Click("Set Status"); err != nil {
		t.Fatal(err)
	}
	if b := h.called("PUT /api/v4/users/me/status/custom"); b["emoji"] != "calendar" || b["text"] != "In a meeting" {
		t.Errorf("sent %v", b)
	}
	if cs := h.srv.Me().CustomStatus; !cs.Shows(time.Now()) || cs.Text != "In a meeting" {
		t.Errorf("custom status %+v", cs)
	}
}

// A DM keeps its menu to what applies to one.
func TestDMMenu(t *testing.T) {
	h := newHarness(t)
	if err := h.tt.RightClick("bob"); err != nil {
		t.Fatal(err)
	}
	items := h.tt.Menu()
	if slices.Contains(items, "Leave Channel") || slices.Contains(items, "Add Members") || !slices.Contains(items, "Mute Conversation") {
		t.Errorf("DM menu %q", items)
	}
	h.tt.CloseMenu()
}
