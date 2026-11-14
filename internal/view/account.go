package view

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// clearAfter is a choice of when a status ends, as the webapp offers them.
type clearAfter struct {
	label    string
	duration string // the webapp's name for it, sent with a custom status
	until    func(now time.Time) time.Time
}

func endOfDay(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 23, 59, 59, 0, now.Location())
}

var (
	never     = clearAfter{label: "Don't clear", until: func(time.Time) time.Time { return time.Time{} }}
	inMinutes = func(label, duration string, d time.Duration) clearAfter {
		return clearAfter{label, duration, func(now time.Time) time.Time { return now.Add(d) }}
	}
	// dndEnds are the choices of the Do not disturb submenu.
	dndEnds = []clearAfter{
		never,
		inMinutes("30 mins", "", 30*time.Minute),
		inMinutes("1 hour", "", time.Hour),
		inMinutes("2 hours", "", 2*time.Hour),
		{label: "Tomorrow", until: func(now time.Time) time.Time {
			y, m, d := now.AddDate(0, 0, 1).Date()
			return time.Date(y, m, d, 9, 0, 0, 0, now.Location())
		}},
	}
	// customEnds are the choices of when a custom status clears.
	customEnds = []clearAfter{
		never,
		inMinutes("30 minutes", "thirty_minutes", 30*time.Minute),
		inMinutes("1 hour", "one_hour", time.Hour),
		inMinutes("4 hours", "four_hours", 4*time.Hour),
		{label: "Today", duration: "today", until: endOfDay},
		{label: "This week", duration: "this_week", until: func(now time.Time) time.Time {
			return endOfDay(now.AddDate(0, 0, (7-int(now.Weekday()))%7))
		}},
	}
	// suggestedStatuses are the webapp's suggestions for a custom status.
	suggestedStatuses = []store.CustomStatus{
		{Emoji: "calendar", Text: "In a meeting"},
		{Emoji: "hamburger", Text: "Out for lunch"},
		{Emoji: "sneezing_face", Text: "Out sick"},
		{Emoji: "house", Text: "Working from home"},
		{Emoji: "palm_tree", Text: "On a vacation"},
	}
	statusChoices = []struct {
		status store.Status
		label  string
	}{{store.Online, "Online"}, {store.Away, "Away"}, {store.DND, "Do not disturb"}, {store.Offline, "Offline"}}
)

// statusIcon draws a status as the account menu shows it.
func statusIcon(c *ui.Context, st store.Status, p palette) {
	ui.Box(c).Size(16, 16).Center().Children(func() {
		if st == store.Offline {
			ui.Box(c).Size(14, 14).Radius(7).BorderWidth(2).BorderColor(c.Theme().TextMuted.Alpha(0.5))
			return
		}
		ui.Box(c).Size(14, 14).Radius(7).Background(statusColor(st, p))
	})
}

// accountMenu is the menu of the top bar's avatar, as the webapp's.
func (v *View) accountMenu(m *ui.Menu, c *ui.Context, s *app.Server) {
	a := v.App
	me := s.Me()
	if me == nil {
		return
	}
	p := colors(c)
	header := m.Item("@" + me.Username).ID("me").Content(func(x ui.MenuItemContent) {
		ui.Row(x.Context).Gap(12).Padding(4, 0).AlignItems(ui.Center).Children(func() {
			v.avatar(x.Context, s, me.ID, 40)
			ui.Column(x.Context).Children(func() {
				if n := me.FullName(); n != "" {
					ui.Text(x.Context, n).FontSize(15).FontWeight(600).TextColor(x.Text)
				}
				ui.Text(x.Context, "@"+me.Username).FontSize(14).TextColor(x.Muted)
			})
		})
	})
	if header.Chosen() {
		v.openProfile(me)
	}
	m.Separator()
	cs := me.CustomStatus
	if cs.Shows(c.Now()) {
		label := emojiText(cs.Emoji) + "  " + cs.Text
		if !cs.ExpiresAt.IsZero() {
			label += "  (until " + cs.ExpiresAt.Local().Format("Jan 2, 15:04") + ")"
		}
		if m.Item(label).ID("custom").Chosen() {
			v.openCustomStatus(cs)
		}
		if m.Item("Clear custom status").Chosen() {
			a.SetCustomStatus(s, nil, "")
		}
	} else if iconItem(m, "Set custom status", "face.smiling").ID("custom").Chosen() {
		v.openCustomStatus(nil)
	}
	m.Separator()
	current := s.Store.Statuses[me.ID]
	for _, sc := range statusChoices {
		icon := func(x ui.MenuItemContent) {
			ui.Row(x.Context).Gap(10).AlignItems(ui.Center).Children(func() {
				statusIcon(x.Context, sc.status, p)
				ui.Column(x.Context).Children(func() {
					ui.Text(x.Context, sc.label).TextColor(x.Text)
					if sc.status == store.DND {
						ui.Text(x.Context, "Disables all notifications").FontSize(12).TextColor(x.Muted)
					}
				})
			})
		}
		if sc.status == store.DND {
			m.Submenu(sc.label, func(m *ui.Menu) {
				for _, end := range dndEnds {
					if m.Item(end.label).Chosen() {
						a.SetStatus(s, store.DND, end.until(time.Now()))
					}
				}
			}).ID(string(sc.status)).Checked(current == sc.status).Content(icon)
			continue
		}
		if m.Item(sc.label).ID(string(sc.status)).Checked(current == sc.status).Content(icon).Chosen() {
			a.SetStatus(s, sc.status, time.Time{})
		}
	}
	m.Separator()
	if iconItem(m, "Profile", "person").Chosen() {
		v.openProfile(me)
	}
	m.Separator()
	if iconItem(m, "Log out", "rectangle.portrait.and.arrow.right").Chosen() {
		a.Logout(s)
	}
}

// iconItem is a menu item with a symbol before its label.
func iconItem(m *ui.Menu, label, sym string) *ui.MenuItem {
	return m.Item(label).Content(func(x ui.MenuItemContent) {
		ui.Row(x.Context).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(x.Context).Size(16, 16).Center().Children(func() { symbol(x.Context, sym, 14, x.Muted) })
			ui.Text(x.Context, label).TextColor(x.Text)
		})
	})
}

type customStatusDialog struct {
	emoji, text string
	end         int
}

type profileDialog struct {
	patch mm.UserPatch
	busy  bool
}

func (d *customStatusDialog) show(v *View, c *ui.Context, s *app.Server) {
	v.customStatusDialog(c, s, d)
}
func (d *profileDialog) show(v *View, c *ui.Context, s *app.Server) { v.profileDialog(c, s, d) }

func (v *View) openProfile(me *store.User) {
	v.dialog = &profileDialog{patch: mm.UserPatch{FirstName: me.FirstName, LastName: me.LastName, Nickname: me.Nickname, Position: me.Position}}
}

func (v *View) openCustomStatus(cs *store.CustomStatus) {
	d := &customStatusDialog{emoji: "speech_balloon"}
	if cs != nil {
		d.emoji, d.text = cs.Emoji, cs.Text
	}
	v.dialog = d
}

func (v *View) customStatusDialog(c *ui.Context, s *app.Server, d *customStatusDialog) {
	t := c.Theme()
	ui.Column(c).Width(460).Padding(8).Gap(10).Children(func() {
		dialogTitle(c, "Set a status")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			pick := ui.ButtonBase(c).Size(40, 40).Radius(4).Center().BorderWidth(1).BorderColor(t.Border).Label("Choose an emoji")
			pick.Children(func() { ui.Text(c, emojiText(d.emoji)).FontSize(20) })
			if pick.Clicked() {
				v.picker = emojiPicker{open: true, inDialog: true, insert: func(name string) { d.emoji = name }}
			}
			if v.picker.open && v.picker.inDialog {
				v.emojiPopover(c, pick, s)
			}
			ui.TextInput(c.Key("text"), &d.text).Placeholder("What is your status?").Label("Status").MaxLength(100).AutoFocus().Grow(1)
		})
		if strings.TrimSpace(d.text) == "" {
			ui.Text(c, "SUGGESTIONS").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Margin(6, 0, 0, 0)
			for _, sg := range suggestedStatuses {
				row := ui.ButtonBase(c.Key(sg.Text)).FillWidth().Padding(6, 8).Radius(4).Label(sg.Text)
				if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, emojiText(sg.Emoji)).FontSize(16)
						ui.Text(c, sg.Text)
					})
				})
				if row.Clicked() {
					d.emoji, d.text = sg.Emoji, sg.Text
				}
			}
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
			ui.Text(c, "Clear after").FontSize(13).FontWeight(600)
			labels := make([]string, len(customEnds))
			for i, e := range customEnds {
				labels[i] = e.label
			}
			sel := labels[d.end]
			ui.Select(c.Key("end"), &sel, labels).Label("Clear after")
			for i, l := range labels {
				if l == sel {
					d.end = i
				}
			}
		})
		v.dialogButtons(c, "Set Status", strings.TrimSpace(d.text) == "", false, func() {
			v.dialog = nil
			end := customEnds[d.end]
			v.App.SetCustomStatus(s, &store.CustomStatus{Emoji: d.emoji, Text: strings.TrimSpace(d.text), ExpiresAt: end.until(time.Now())}, end.duration)
		})
	})
}

func (v *View) profileDialog(c *ui.Context, s *app.Server, d *profileDialog) {
	t := c.Theme()
	me := s.Me()
	if me == nil {
		v.dialog = nil
		return
	}
	ui.Column(c).Width(460).Padding(8).Gap(6).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(0, 0, 8, 0).Children(func() {
			v.avatar(c, s, me.ID, 56)
			ui.Column(c).Children(func() {
				dialogTitle(c, "Profile")
				ui.Text(c, "@"+me.Username).TextColor(t.TextMuted)
				if me.Email != "" {
					ui.Text(c, me.Email).FontSize(12).TextColor(t.TextMuted)
				}
				if ui.Link(c, "Change picture", "").FontSize(12).Clicked() {
					v.changePicture(s)
				}
			})
		})
		first := field(c, "first", "First name", &d.patch.FirstName, "").AutoFocus()
		last := field(c, "last", "Last name", &d.patch.LastName, "")
		nick := field(c, "nick", "Nickname", &d.patch.Nickname, "")
		pos := field(c, "pos", "Position", &d.patch.Position, "")
		submitted := first.Submitted() || last.Submitted() || nick.Submitted() || pos.Submitted()
		v.dialogButtons(c, "Save", d.busy, submitted, func() {
			d.busy = true
			v.App.PatchMe(s, d.patch, func() {
				if v.dialog == d {
					v.dialog = nil
				}
			})
		})
	})
}
