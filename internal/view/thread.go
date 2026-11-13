package view

import (
	"cmp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// thread is the RHS: a thread and a composer for replies.
func (v *View) thread(c *ui.Context, s *app.Server) {
	t := c.Theme()
	root := s.Thread
	ui.Column(c.Key(root)).Grow(1).FillHeight().Background(t.Background).Children(func() {
		ui.Row(c).Height(62).Padding(0, 12, 0, 16).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			if v.currentPanel(s) != nil && iconButton(c, "chevron.left", "Back", 13, t.TextMuted, t.SurfaceHover).Clicked() {
				v.App.CloseThread(s)
			}
			ui.Text(c, "Thread").FontWeight(700).FontSize(16)
			chID := s.Channel
			if p := s.Store.Posts[root]; p != nil {
				chID = p.ChannelID
			}
			if ch := s.Store.Channels[chID]; ch != nil {
				ui.Text(c, v.App.ChannelName(s, ch)).FontSize(12).TextColor(t.TextMuted).MaxLines(1).Shrink(1).MinWidth(0)
			}
			ui.Spacer(c)
			_, following := s.Store.Followed[root]
			label := "Follow"
			if following {
				label = "Following"
			}
			follow := ui.ButtonBase(c).Padding(3, 8).Radius(4).BorderWidth(1).BorderColor(t.Border).Shrink(0).Label(label).Tooltip("Follow the thread to see it in Threads")
			if follow.Hovered() {
				follow.Background(t.SurfaceHover)
			}
			follow.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(600) })
			if follow.Clicked() {
				v.App.SetFollowing(s, root, !following)
			}
			if iconButton(c, "rectangle.portrait.on.rectangle.portrait", "Open in new window", 14, t.TextMuted, t.SurfaceHover).Clicked() {
				v.popOut(s, chID, root)
			}
			if iconButton(c, "xmark", "Close thread", 14, t.TextMuted, t.SurfaceHover).Clicked() {
				v.App.CloseThread(s)
			}
		})
		p := s.Store.Posts[root]
		ch := s.Channel
		if p != nil {
			ch = p.ChannelID
		}
		v.postList(c, s, ch, root)
		v.composer(c, s, ch, root)
	})
}

// switcher is the Cmd+K dialog that finds channels and people.
type switcher struct {
	open   bool
	query  string
	sel    int
	asked  string
	result []switchItem
}

type switchItem struct {
	label, detail string
	channel       store.ChannelID
	user          store.UserID
	unread        bool
}

func (v *View) switchItems(s *app.Server) []switchItem {
	q := strings.ToLower(strings.TrimSpace(v.switcher.query))
	q = strings.TrimLeft(q, "@~#")
	var items []switchItem
	dms := map[store.UserID]bool{}
	for id, ch := range s.Store.Channels {
		name := v.App.ChannelName(s, ch)
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(ch.Name, q) {
			continue
		}
		detail := ""
		if t := s.Store.Teams[ch.TeamID]; t != nil && len(s.Store.TeamOrder) > 1 {
			detail = t.DisplayName
		}
		if ch.Type == store.Direct {
			dms[ch.DMPartner(s.Store.Me)] = true
			detail = "Direct message"
		}
		items = append(items, switchItem{label: name, detail: detail, channel: id, unread: s.Store.Unread(id)})
	}
	if q != "" {
		for id, u := range s.Store.Users {
			if dms[id] || id == s.Store.Me || u.Deleted {
				continue
			}
			if strings.HasPrefix(u.Username, q) || strings.Contains(strings.ToLower(u.FullName()), q) {
				items = append(items, switchItem{label: u.Username, detail: u.FullName(), user: id})
			}
		}
	}
	slices.SortStableFunc(items, func(a, b switchItem) int {
		if a.unread != b.unread {
			if a.unread {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.label), strings.ToLower(b.label))
	})
	return items[:min(len(items), 50)]
}

func (v *View) switcherDialog(c *ui.Context) {
	sw := &v.switcher
	s := v.App.Current
	if s == nil || !s.Ready {
		sw.open = false
	}
	ui.Modal(c, &sw.open, func() {
		t := c.Theme()
		items := v.switchItems(s)
		if q := strings.TrimLeft(strings.TrimSpace(sw.query), "@~#"); len(q) >= 2 && q != sw.asked {
			sw.asked = q
			v.App.SearchUsers(s, q, func([]store.User) {})
		}
		sw.sel = min(sw.sel, max(len(items)-1, 0))
		choose := func(it switchItem) {
			sw.open = false
			if it.user != "" {
				v.App.OpenDM(s, it.user)
			} else {
				v.App.OpenChannel(s, it.channel)
			}
		}
		ui.Column(c).Width(520).Gap(8).Padding(4).Children(func() {
			ui.Text(c, "Find Channels").Bold().FontSize(16)
			in := ui.TextInput(c.Key("q"), &sw.query).Placeholder("Type to find a channel or a person").Label("Find").AutoFocus().
				OnTextKey(func(k ui.TextKey) bool {
					switch k.Key {
					case ui.KeyUp:
						sw.sel = max(sw.sel-1, 0)
						return true
					case ui.KeyDown:
						sw.sel++
						return true
					}
					return false
				})
			if in.Changed() {
				sw.sel = 0
			}
			if in.Submitted() && len(items) > 0 {
				choose(items[sw.sel])
				return
			}
			ui.Column(c).MaxHeight(360).Children(func() {
				ui.Scroll(c).Children(func() {
					for i, it := range items {
						row := ui.ButtonBase(c.Key(i)).FillWidth().Padding(6, 10).Radius(4).Label(it.label)
						if i == sw.sel {
							row.Background(t.Selection)
						}
						row.Children(func() {
							ui.Row(c).Gap(8).Children(func() {
								txt := ui.Text(c, it.label)
								if it.unread {
									txt.Bold()
								}
								if it.detail != "" {
									ui.Text(c, it.detail).TextColor(t.TextMuted).FontSize(12)
								}
							})
						})
						if row.Clicked() {
							choose(it)
						}
					}
				})
			})
		})
	})
}
