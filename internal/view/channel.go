package view

import (
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

func (v *View) channel(c *ui.Context, s *app.Server) {
	t := c.Theme()
	ch := s.Store.Channels[s.Channel]
	ui.Column(c).Grow(1).FillHeight().Background(t.Background).Children(func() {
		if ch == nil {
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Text(c, "No channel open").TextColor(t.TextMuted)
			})
			return
		}
		v.channelHeader(c, s, ch)
		ui.Column(c.Key(ch.ID)).Grow(1).Children(func() {
			v.postList(c, s, ch.ID, "")
			if ch.Deleted {
				ui.Row(c).Margin(0, 20, 16, 20).Padding(14).Radius(8).Background(t.Surface).Gap(8).AlignItems(ui.Center).Children(func() {
					symbol(c, "archivebox", 14, t.TextMuted)
					ui.Text(c, "You are viewing an archived channel. No one can post in it.").TextColor(t.TextMuted).Grow(1)
					if ui.Button(c, "Unarchive").Clicked() {
						v.App.RestoreChannel(s, ch.ID)
					}
				})
				return
			}
			v.composer(c, s, ch.ID, "")
		})
	})
}

func (v *View) channelHeader(c *ui.Context, s *app.Server, ch *store.Channel) {
	t := c.Theme()
	p := colors(c)
	ui.Row(c).Height(62).Padding(0, 16, 0, 20).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		switch ch.Type {
		case store.Open:
			symbol(c, "globe", 15, t.TextMuted)
		case store.Private:
			symbol(c, "lock", 15, t.TextMuted)
		case store.Direct:
			v.avatar(c, s, ch.DMPartner(s.Store.Me), 24)
		}
		name := ui.ButtonBase(c).Padding(4, 6).Margin(0, 0, 0, -6).Radius(4).Shrink(0).Label(v.App.ChannelName(s, ch) + " channel menu").Tooltip("Channel menu")
		if name.Hovered() {
			name.Background(t.SurfaceHover)
		}
		name.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, v.App.ChannelName(s, ch)).FontWeight(700).FontSize(16).MaxLines(1).Role(ui.RoleHeading)
				symbol(c, "chevron.down", 10, t.TextMuted)
			})
		}).Menu(func(m *ui.Menu) { v.channelMenu(m, c, s, ch.ID, true) })
		if ch.Type == store.Direct {
			v.statusDot(c, s.Store.Statuses[ch.DMPartner(s.Store.Me)], p)
		}
		if ch.MemberCount > 0 && ch.Type != store.Direct {
			members := ui.ButtonBase(c).Padding(2, 6).Radius(4).Label("View members").Tooltip(fmt.Sprint(ch.MemberCount, " members"))
			if members.Hovered() {
				members.Background(t.SurfaceHover)
			}
			members.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					symbol(c, "person.2", 13, t.TextMuted)
					ui.Text(c, fmt.Sprint(ch.MemberCount)).FontSize(12).TextColor(t.TextMuted)
				})
			})
			if members.Clicked() && ch.Type != store.Group {
				v.openMembers(s, ch.ID)
			}
		}
		about := ch.Purpose
		if about == "" {
			about = ch.Header
		}
		if about != "" {
			ui.Box(c).Width(1).Height(16).Background(t.Border).Margin(0, 4)
			ui.Text(c, about).TextColor(t.TextMuted).FontSize(13).MaxLines(1).Grow(1).Shrink(1).Tooltip(about)
		} else {
			ui.Spacer(c)
		}
		if iconButton(c, "pin", "Pinned messages", 14, t.TextMuted, t.SurfaceHover).Clicked() {
			v.openPinned(s, ch.ID)
		}
		if iconButton(c, "info.circle", "Channel info", 14, t.TextMuted, t.SurfaceHover).Clicked() {
			v.openInfo(s, ch.ID, 0)
		}
		if iconButton(c, "link", "Copy channel link", 14, t.TextMuted, t.SurfaceHover).Clicked() {
			if team := s.Store.Teams[s.Team]; team != nil {
				c.WriteClipboard(s.URL.String() + "/" + team.Name + "/channels/" + ch.Name)
				c.Toast("Link copied")
			}
		}
	})
}

// postList shows a channel's posts, or a thread's when root is set.
func (v *View) postList(c *ui.Context, s *app.Server, ch store.ChannelID, root store.PostID) {
	t := c.Theme()
	var tl *store.Timeline
	key := "ch:" + string(ch)
	if root != "" {
		tl = s.Store.Threads[root]
		key = "th:" + string(root)
	} else {
		tl = s.Store.Timelines[ch]
	}
	var order []store.PostID
	if tl != nil {
		order = tl.Order
	}
	l := v.list(s, key)
	l.Key = func(i int) any { return postKey(s.Store.Posts[order[i]]) }
	l.FollowEnd = true
	// A post jumped to scrolls into the middle, once for each list.
	if j := s.Jump; !j.At.IsZero() && v.jumped[key] != j.At {
		target := j.Root
		if root != "" {
			target = j.Post
		}
		if i := slices.Index(order, target); i >= 0 {
			v.jumped[key] = j.At
			l.ScrollTo(i, ui.Center)
		}
	}
	if root == "" && tl != nil && tl.HasOlder && !tl.Loading {
		// Before its first layout a list shows no rows (last is 0).
		if first, last := l.Visible(); last > 0 && first < 5 {
			v.App.LoadOlder(s, ch)
		}
	}
	ui.Box(c).Grow(1).Children(func() {
		// A channel's first posts sit at its bottom, as a chat's do; a
		// thread reads from its top, as the webapp's RHS.
		justify := ui.End
		if root != "" {
			justify = ui.Start
		}
		ui.List(c, l, len(order)).Fill().Padding(12, 0, 4, 0).Justify(justify).Rows(func(row ui.ListRow) {
			if row.Index == 0 && root == "" && tl.Loading && tl.HasOlder {
				ui.Row(row.Context).Center().Padding(8).Children(func() { ui.Spinner(row.Context) })
			}
			v.postRow(row, s, order, row.Index, root)
		}).Children(func() {
			if len(order) == 0 {
				ui.Column(c).Grow(1).Center().Children(func() {
					if tl == nil || tl.Loading {
						ui.Spinner(c)
					} else {
						ui.Text(c, "No posts yet").TextColor(t.TextMuted)
					}
				})
			}
		})
		if !l.AtEnd() && len(order) > 0 {
			p := colors(c)
			ui.Row(c).Absolute().Bottom(12).Left(0).Right(0).Justify(ui.Center).Children(func() {
				jump := ui.ButtonBase(c).Height(32).Padding(0, 14).Radius(16).Background(p.buttonBg).Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.16)).Label("Jump to recent messages")
				jump.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						symbol(c, "arrow.down", 11, ui.Hex("#ffffff"))
						ui.Text(c, "Jump to recent messages").FontSize(13).FontWeight(600).TextColor(ui.Hex("#ffffff"))
					})
				})
				if jump.Clicked() {
					l.ScrollToEnd()
				}
			})
		}
	})
	v.typing(c, s, ch, root)
}

func day(ms int64) time.Time {
	t := time.UnixMilli(ms)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dayLabel(d, today time.Time) string {
	switch {
	case d.Equal(today):
		return "Today"
	case d.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	case d.Year() == today.Year():
		return d.Format("Monday, January 2")
	default:
		return d.Format("January 2, 2006")
	}
}

// separator is a line across the list with a label in its middle.
func separator(c *ui.Context, label string, color, line ui.Color) {
	ui.Row(c).Padding(10, 20).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(line)
		ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(color)
		ui.Box(c).Height(1).Grow(1).Background(line)
	})
}

func (v *View) typing(c *ui.Context, s *app.Server, ch store.ChannelID, root store.PostID) {
	t := c.Theme()
	if root != "" {
		ui.Box(c).Height(4)
		return
	}
	users, next := s.Store.Typers(ch, c.Now())
	users = slices.DeleteFunc(users, func(u store.UserID) bool { return u == s.Store.Me })
	text := ""
	switch len(users) {
	case 0:
	case 1:
		text = v.App.UserName(s, users[0]) + " is typing…"
	case 2:
		text = v.App.UserName(s, users[0]) + " and " + v.App.UserName(s, users[1]) + " are typing…"
	default:
		text = "Several people are typing…"
	}
	if !next.IsZero() {
		c.After(next.Sub(c.Now()))
	}
	ui.Text(c, text).FontSize(11).TextColor(t.TextMuted).Padding(0, 24).Height(18)
}
