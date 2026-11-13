package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// menuRow is a row of a popover menu; it reports a click.
func menuRow(c *ui.Context, key any, label, detail string, danger bool) bool {
	t := c.Theme()
	row := ui.ButtonBase(c.Key(key)).FillWidth().Padding(6, 16).Label(label)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			txt := ui.Text(c, label).FontSize(14).Grow(1)
			if danger {
				txt.TextColor(t.Danger)
			}
			if detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
	return row.Clicked()
}

func menuPanel(c *ui.Context, width float32, fn func()) {
	ui.Column(c).Width(width).Padding(8, 0).Children(fn)
}

func menuDivider(c *ui.Context) {
	ui.Box(c).Height(1).FillWidth().Margin(8, 0).Background(c.Theme().Border)
}

func (v *View) topBar(c *ui.Context) {
	p := colors(c)
	a := v.App
	s := a.Current
	bar := c.TitleBar()
	white := ui.Hex("#ffffff")
	ui.Row(c).Height(max(bar.Height, 40)).Padding(0, 8+bar.Right, 0, 8+bar.Left).Gap(4).AlignItems(ui.Center).Background(p.topBar).DragWindow().Children(func() {
		ui.Row(c).Width(260).Gap(4).AlignItems(ui.Center).Children(func() {
			v.serverButton(c, s, white)
			if _, in := s.Session.(app.LoggedIn); in && s.Ready {
				if iconButton(c, "chevron.left", "Back", 13, p.sidebarText, p.sidebarHover).Clicked() {
					a.Back(s)
				}
				if iconButton(c, "chevron.right", "Forward", 13, p.sidebarText, p.sidebarHover).Clicked() {
					a.Forward(s)
				}
			}
		})
		ui.Row(c).Grow(1).Justify(ui.Center).Children(func() {
			if _, in := s.Session.(app.LoggedIn); !in || !s.Ready {
				return
			}
			find := ui.ButtonBase(c).Width(480).MaxWidth(480).Height(28).Padding(0, 10).Radius(4).Label("Find channels and people")
			bg := ui.RGBA(255, 255, 255, 0.08)
			if find.Hovered() {
				bg = ui.RGBA(255, 255, 255, 0.16)
			}
			find.Background(bg).Children(func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					symbol(c, "magnifyingglass", 13, p.sidebarText)
					ui.Text(c, "Find channels and people").FontSize(13).TextColor(p.sidebarText).Grow(1)
					ui.Text(c, "⌘K").FontSize(12).TextColor(p.offline)
				})
			})
			if find.Clicked() {
				v.switcher = switcher{open: true}
			}
		})
		ui.Row(c).Width(260).Gap(8).Justify(ui.End).AlignItems(ui.Center).Children(func() {
			if _, in := s.Session.(app.LoggedIn); in {
				v.connState(c, s, p)
				if s.Ready {
					v.meButton(c, s)
				}
			}
		})
	})
}

func (v *View) serverButton(c *ui.Context, s *app.Server, white ui.Color) {
	p := colors(c)
	a := v.App
	btn := ui.ButtonBase(c).Height(28).Padding(0, 8).Radius(4).Label("Servers").Tooltip(s.URL.String())
	if btn.Hovered() || v.serverMenu {
		btn.Background(p.sidebarHover)
	}
	btn.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, initials(s.Config.Name)).FontSize(10).Bold().Size(20, 20).TextAlign(ui.Center).Padding(3, 0).Radius(4).Background(ui.RGBA(255, 255, 255, 0.24)).TextColor(white)
			ui.Text(c, s.Config.Name).FontSize(14).FontWeight(600).TextColor(white).MaxLines(1).MaxWidth(150)
			if m, u := otherServersBadge(a, s); m > 0 {
				ui.Text(c, count(m)).FontSize(10).Bold().Padding(1, 5).Radius(8).Background(p.dnd).TextColor(white)
			} else if u {
				ui.Box(c).Size(6, 6).Radius(3).Background(white)
			}
			symbol(c, "chevron.down", 9, p.sidebarText)
		})
	})
	if btn.Clicked() {
		v.serverMenu = !v.serverMenu
	}
	ui.Popover(c, btn, &v.serverMenu, func() {
		menuPanel(c, 280, func() {
			ui.Text(c, "SERVERS").FontSize(11).Bold().TextColor(c.Theme().TextMuted).Padding(4, 16)
			for _, other := range a.Servers {
				detail := ""
				if n, u := serverBadge(other); n > 0 {
					detail = count(n)
				} else if u {
					detail = "•"
				}
				label := other.Config.Name
				if other == s {
					label = "✓  " + label
				}
				if menuRow(c, other.Config.ID, label, detail, false) {
					v.serverMenu = false
					a.SelectServer(other)
				}
			}
			menuDivider(c)
			if menuRow(c, "add", "Add Server…", "", false) {
				v.serverMenu = false
				v.adding, v.addURL, v.addName, v.addError = true, "", "", ""
			}
			if menuRow(c, "edit", "Edit "+s.Config.Name+"…", "", false) {
				v.serverMenu = false
				v.editServer, v.editName = s, s.Config.Name
			}
			if menuRow(c, "remove", "Remove "+s.Config.Name, "", true) {
				v.serverMenu = false
				a.RemoveServer(s)
			}
		})
	})
}

// otherServersBadge is what the servers other than s have unread, which
// the server button shows as the official client's tabs do.
func otherServersBadge(a *app.App, s *app.Server) (int64, bool) {
	var mentions int64
	unread := false
	for _, other := range a.Servers {
		if other == s {
			continue
		}
		m, u := serverBadge(other)
		mentions += m
		unread = unread || u
	}
	return mentions, unread
}

func (v *View) connState(c *ui.Context, s *app.Server, p palette) {
	switch conn := s.Store.Conn.(type) {
	case store.ConnConnecting:
		if s.Ready {
			ui.Text(c, "Connecting…").FontSize(12).TextColor(p.sidebarText)
		}
	case store.ConnOffline:
		wait := time.Until(conn.Retry).Round(time.Second)
		msg := "Offline"
		if wait > 0 {
			msg = fmt.Sprintf("Offline · retrying in %s", wait)
			c.After(time.Second)
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 8).Radius(4).Background(ui.RGBA(255, 188, 31, 0.16)).Tooltip(conn.Err).Children(func() {
			symbol(c, "wifi.slash", 11, p.away)
			ui.Text(c, msg).FontSize(12).TextColor(p.away)
		})
	}
}

func (v *View) meButton(c *ui.Context, s *app.Server) {
	p := colors(c)
	me := s.Me()
	if me == nil {
		return
	}
	btn := ui.ButtonBase(c).Size(32, 32).Radius(16).Center().Label("Account").Tooltip(me.Username)
	btn.Children(func() {
		ui.Box(c).Size(28, 28).Children(func() {
			v.avatar(c, s, me.ID, 28)
			ui.Box(c).Absolute().Right(-2).Top(17).Size(12, 12).Radius(6).Background(p.topBar).Center().Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(statusColor(s.Store.Statuses[me.ID], p))
			})
		})
	})
	if btn.Clicked() {
		v.meMenu = !v.meMenu
	}
	ui.Popover(c, btn, &v.meMenu, func() {
		menuPanel(c, 240, func() {
			ui.Column(c).Padding(4, 16, 8, 16).Children(func() {
				ui.Text(c, me.FullName()).Bold().FontSize(14)
				ui.Text(c, "@"+me.Username).FontSize(12).TextColor(c.Theme().TextMuted)
			})
			menuDivider(c)
			if menuRow(c, "logout", "Log Out", "", true) {
				v.meMenu = false
				v.App.Logout(s)
			}
		})
	})
}

func statusColor(st store.Status, p palette) ui.Color {
	switch st {
	case store.Online:
		return p.online
	case store.Away:
		return p.away
	case store.DND:
		return p.dnd
	}
	return p.offline
}
