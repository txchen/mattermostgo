package view

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

func (v *View) teamStrip(c *ui.Context, s *app.Server) {
	p := colors(c)
	ui.Column(c).Width(64).FillHeight().Padding(12, 0).Gap(12).AlignItems(ui.Center).Background(p.teamBar).Children(func() {
		for _, id := range s.Store.TeamOrder {
			team := s.Store.Teams[id]
			mentions, unread := teamBadge(s, id)
			current := id == s.Team
			ui.Box(c.Key(id)).Children(func() {
				btn := ui.ButtonBase(c).Size(40, 40).Radius(8).Background(ui.RGBA(255, 255, 255, 0.16)).Center().Label(team.DisplayName).Tooltip(team.DisplayName)
				if current {
					btn.BorderWidth(2).BorderColor(p.sidebarTextStrong)
				} else {
					btn.Opacity(0.64)
				}
				btn.Children(func() {
					ui.Text(c, initials(team.DisplayName)).Bold().TextColor(p.sidebarTextStrong)
				})
				if btn.Clicked() {
					v.App.SelectTeam(s, id)
				}
				if mentions > 0 {
					ui.Text(c, count(mentions)).FontSize(10).Bold().Padding(1, 5).Radius(8).Background(p.dnd).TextColor(ui.Hex("#ffffff")).Absolute().Top(-6).Right(-8)
				} else if unread && !current {
					ui.Box(c).Size(8, 8).Radius(4).Background(p.sidebarTextStrong).Absolute().Top(16).Left(-12)
				}
			})
		}
	})
}

func teamBadge(s *app.Server, team store.TeamID) (int64, bool) {
	var mentions int64
	unread := false
	for id, ch := range s.Store.Channels {
		if ch.TeamID != team {
			continue
		}
		mentions += s.Store.Mentions(id)
		if m := s.Store.Members[id]; m != nil && !m.Muted() && s.Store.Unread(id) {
			unread = true
		}
	}
	return mentions, unread
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		r, _ := utf8.DecodeRuneInString(w)
		out = append(out, unicode.ToUpper(r))
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

func count(n int64) string {
	if n > 99 {
		return "99+"
	}
	return fmt.Sprint(n)
}

// sidebarGroup is a category as the sidebar shows it, its channels sorted.
type sidebarGroup struct {
	key      string
	title    string
	channels []store.ChannelID
}

// sidebarGroups returns the current team's categories with their
// channels. Channels in no category go to the default category of their
// type, so that a channel the server has not filed yet still shows.
func (v *View) sidebarGroups(s *app.Server) []sidebarGroup {
	st := s.Store
	inTeam := func(ch *store.Channel) bool { return ch.TeamID == s.Team || ch.TeamID == "" }
	placed := map[store.ChannelID]bool{}
	var groups []sidebarGroup
	dmIndex, chIndex := -1, -1
	for _, cat := range st.Categories[s.Team] {
		g := sidebarGroup{key: cat.ID, title: cat.DisplayName}
		for _, id := range cat.Channels {
			if ch := st.Channels[id]; ch != nil && inTeam(ch) && !placed[id] {
				g.channels = append(g.channels, id)
				placed[id] = true
			}
		}
		// An empty Favorites hides, as the webapp's.
		if cat.Type == store.CategoryFavorites && len(g.channels) == 0 {
			continue
		}
		switch cat.Type {
		case store.CategoryDirect:
			dmIndex = len(groups)
		case store.CategoryChannels:
			chIndex = len(groups)
		}
		groups = append(groups, g)
		v.sortGroup(s, cat.Sorting, cat.Type, &groups[len(groups)-1])
	}
	if chIndex < 0 {
		chIndex = len(groups)
		groups = append(groups, sidebarGroup{key: "channels", title: "Channels"})
	}
	if dmIndex < 0 {
		dmIndex = len(groups)
		groups = append(groups, sidebarGroup{key: "dms", title: "Direct Messages"})
	}
	var extra []store.ChannelID
	for id, ch := range st.Channels {
		if !placed[id] && inTeam(ch) {
			extra = append(extra, id)
		}
	}
	slices.Sort(extra)
	for _, id := range extra {
		ch := st.Channels[id]
		if ch.Type == store.Direct || ch.Type == store.Group {
			groups[dmIndex].channels = append(groups[dmIndex].channels, id)
		} else {
			groups[chIndex].channels = append(groups[chIndex].channels, id)
		}
	}
	if len(extra) > 0 {
		v.sortGroup(s, "", store.CategoryDirect, &groups[dmIndex])
		v.sortGroup(s, "alpha", store.CategoryChannels, &groups[chIndex])
	}
	return groups
}

func (v *View) sortGroup(s *app.Server, sorting string, kind store.CategoryType, g *sidebarGroup) {
	st := s.Store
	if sorting == "" {
		sorting = "alpha"
		if kind == store.CategoryDirect {
			sorting = "recent"
		}
	}
	switch sorting {
	case "alpha":
		slices.SortStableFunc(g.channels, func(a, b store.ChannelID) int {
			return cmp.Compare(strings.ToLower(v.App.ChannelName(s, st.Channels[a])), strings.ToLower(v.App.ChannelName(s, st.Channels[b])))
		})
	case "recent":
		slices.SortStableFunc(g.channels, func(a, b store.ChannelID) int {
			return cmp.Compare(st.Channels[b].LastPostAt, st.Channels[a].LastPostAt)
		})
	}
}

// sidebarOrder is the channels in the order the sidebar shows them.
func (v *View) sidebarOrder(s *app.Server) []store.ChannelID {
	var out []store.ChannelID
	for _, g := range v.sidebarGroups(s) {
		if !v.collapsed[g.key] {
			out = append(out, g.channels...)
		}
	}
	return out
}

func (v *View) sidebar(c *ui.Context, s *app.Server) {
	p := colors(c)
	ui.Column(c).Width(260).FillHeight().Background(p.sidebar).Children(func() {
		teamName := s.Config.Name
		if t := s.Store.Teams[s.Team]; t != nil {
			teamName = t.DisplayName
		}
		ui.Row(c).Height(52).Padding(0, 12, 0, 16).Gap(6).AlignItems(ui.Center).Children(func() {
			team := ui.ButtonBase(c).Padding(4, 6).Margin(0, 0, 0, -6).Radius(4).Shrink(1).Label("Team menu")
			if team.Hovered() {
				team.Background(p.sidebarHover)
			}
			team.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, teamName).Bold().FontSize(16).TextColor(p.sidebarTextStrong).MaxLines(1).Shrink(1)
					symbol(c, "chevron.down", 10, p.sidebarText)
				})
			}).Menu(func(m *ui.Menu) {
				if m.Item("Create New Channel").Chosen() {
					v.dialog = &createChannelDialog{}
				}
				if m.Item("Browse Channels").Chosen() {
					v.openBrowse(s)
				}
				if m.Item("Create New Category").Chosen() {
					v.newCategory(s, "")
				}
				m.Separator()
				if m.Item("Integrations").Chosen() {
					v.openIntegrations(s)
				}
			})
			ui.Spacer(c)
			plus := ui.ButtonBase(c).Size(28, 28).Radius(14).Center().Label("New").Tooltip("Browse channels or start a DM")
			bg := ui.RGBA(255, 255, 255, 0.08)
			if plus.Hovered() || v.plusMenu {
				bg = ui.RGBA(255, 255, 255, 0.16)
			}
			plus.Background(bg).Children(func() { symbol(c, "plus", 12, p.sidebarTextStrong) })
			if plus.Clicked() {
				v.plusMenu = !v.plusMenu
			}
			ui.Popover(c, plus, &v.plusMenu, func() {
				menuPanel(c, 240, func() {
					if menuRow(c, "create", "Create new channel", "", false) {
						v.plusMenu = false
						v.dialog = &createChannelDialog{}
					}
					if menuRow(c, "browse", "Browse channels", "", false) {
						v.plusMenu = false
						v.openBrowse(s)
					}
					if menuRow(c, "category", "Create new category", "", false) {
						v.plusMenu = false
						v.newCategory(s, "")
					}
					menuDivider(c)
					if menuRow(c, "dm", "Open a direct message", "⌘K", false) {
						v.plusMenu = false
						v.switcher = switcher{open: true, query: "@"}
					}
				})
			})
		})
		find := ui.ButtonBase(c).Margin(0, 12, 8, 12).Height(28).Padding(0, 10).Radius(4).Label("Find channel")
		bg := ui.RGBA(255, 255, 255, 0.08)
		if find.Hovered() {
			bg = ui.RGBA(255, 255, 255, 0.16)
		}
		find.Background(bg).Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				symbol(c, "magnifyingglass", 12, p.sidebarText)
				ui.Text(c, "Find channel").FontSize(13).TextColor(p.sidebarText)
			})
		})
		if find.Clicked() {
			v.switcher = switcher{open: true}
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 0, 16, 0).Children(func() {
				for _, g := range v.sidebarGroups(s) {
					v.sidebarGroup(c, s, g)
				}
			})
		})
	})
}

func (v *View) sidebarGroup(c *ui.Context, s *app.Server, g sidebarGroup) {
	p := colors(c)
	ui.Column(c.Key(g.key)).Children(func() {
		collapsed := v.collapsed[g.key]
		head := ui.ButtonBase(c).Padding(14, 12, 6, 12).FillWidth().Label(g.title)
		menu := func(m *ui.Menu) { v.categoryMenu(m, s, g.key) }
		head.ContextMenu(menu).Children(func() {
			ui.Row(c).FillWidth().Height(16).Gap(4).AlignItems(ui.Center).Children(func() {
				arrow := "chevron.down"
				if collapsed {
					arrow = "chevron.right"
				}
				symbol(c, arrow, 9, p.sidebarText)
				ui.Text(c, strings.ToUpper(g.title)).FontSize(12).FontWeight(600).LetterSpacing(0.3).TextColor(p.sidebarText).Grow(1).MaxLines(1)
				if head.Hovered() {
					more := ui.ButtonBase(c).Size(20, 20).Radius(4).Center().Label("Category options").Tooltip("Category options")
					more.Children(func() { symbol(c, "ellipsis", 11, p.sidebarText) }).Menu(menu)
				}
			})
		})
		if head.Clicked() {
			v.collapsed[g.key] = !collapsed
		}
		for _, id := range g.channels {
			// A collapsed category still shows its unread channels, as the webapp's.
			if collapsed && !s.Store.Unread(id) && id != s.Channel {
				continue
			}
			v.sidebarItem(c.Key(id), s, id)
		}
	})
}

func (v *View) sidebarItem(c *ui.Context, s *app.Server, id store.ChannelID) {
	p := colors(c)
	st := s.Store
	ch := st.Channels[id]
	m := st.Members[id]
	muted := m != nil && m.Muted()
	unread := st.Unread(id) && !muted
	mentions := st.Mentions(id)
	active := id == s.Channel
	name := v.App.ChannelName(s, ch)
	item := ui.ButtonBase(c).FillWidth().Height(32).Padding(0, 10, 0, 20).Label(name)
	menu := func(m *ui.Menu) { v.channelMenu(m, c, s, id, false) }
	item.ContextMenu(menu)
	bg := ui.Transparent
	switch {
	case active:
		bg = p.sidebarActive
	case item.Hovered():
		bg = p.sidebarHover
	}
	item.Background(bg).Children(func() {
		if active {
			ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(4).Background(p.link)
		}
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			color := p.sidebarText
			if unread || active || mentions > 0 {
				color = p.sidebarTextStrong
			}
			if muted {
				color = p.offline
			}
			v.channelIcon(c, s, ch, color, p)
			txt := ui.Text(c, name).MaxLines(1).Grow(1).Shrink(1).FontSize(14).TextColor(color)
			if unread || mentions > 0 {
				txt.FontWeight(600)
			}
			if mentions > 0 {
				ui.Text(c, count(mentions)).FontSize(11).Bold().Padding(1, 6).MinWidth(20).TextAlign(ui.Center).Radius(9).Background(p.mentionBg).TextColor(p.mentionText)
			}
			if item.Hovered() {
				more := ui.ButtonBase(c).Size(22, 22).Radius(4).Center().Label("Channel options for " + name).Tooltip("Channel options")
				if more.Hovered() {
					more.Background(p.sidebarHover)
				}
				more.Children(func() { symbol(c, "ellipsis", 12, p.sidebarTextStrong).Rotate(90) }).Menu(menu)
			} else {
				ui.Box(c).Size(22, 22)
			}
		})
	})
	if item.Clicked() {
		v.App.OpenChannel(s, id)
	}
}

func (v *View) channelIcon(c *ui.Context, s *app.Server, ch *store.Channel, color ui.Color, p palette) {
	switch ch.Type {
	case store.Direct:
		partner := ch.DMPartner(s.Store.Me)
		ui.Box(c).Size(20, 20).Children(func() {
			v.avatar(c, s, partner, 20)
			ui.Box(c).Absolute().Right(-3).Bottom(-3).Size(11, 11).Radius(6).Background(p.sidebar).Center().Children(func() {
				ui.Box(c).Size(7, 7).Radius(4).Background(statusColor(s.Store.Statuses[partner], p))
			})
		})
	case store.Group:
		n := strings.Count(ch.DisplayName, ",") + 1
		ui.Text(c, fmt.Sprint(n)).FontSize(10).Bold().Size(18, 18).Padding(2, 0).TextAlign(ui.Center).Radius(4).Background(ui.RGBA(255, 255, 255, 0.16)).TextColor(color)
	case store.Private:
		symbol(c, "lock", 15, color)
	default:
		symbol(c, "globe", 15, color)
	}
}

func (v *View) statusDot(c *ui.Context, st store.Status, p palette) {
	ui.Box(c).Size(14, 14).Center().Label(string(st)).Children(func() {
		ui.Box(c).Size(9, 9).Radius(5).Background(statusColor(st, p))
	})
}
