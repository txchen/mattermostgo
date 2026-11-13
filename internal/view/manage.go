package view

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// dialog is the one dialog the window shows over the main view, if any.
type dialog interface{ isDialog() }

// textDialog asks for one line of text, or several with lines.
type textDialog struct {
	title, label, value, submit string
	lines                       bool
	save                        func(string)
}

// confirmDialog asks before an action that is hard to undo.
type confirmDialog struct {
	title, body, action string
	do                  func()
}

type renameDialog struct {
	channel          store.ChannelID
	display, urlName string
}

type createChannelDialog struct {
	display, purpose string
	private, busy    bool
	err              string
}

type browseDialog struct {
	query    string
	channels []store.Channel
	loaded   bool
}

type addMembersDialog struct {
	channel store.ChannelID
	query   string
	asked   string
	found   []store.UserID
	picked  []store.UserID
	members map[store.UserID]bool
	busy    bool
}

type membersDialog struct {
	channel store.ChannelID
	members []mm.MemberInfo
	loaded  bool
}

func (*textDialog) isDialog()          {}
func (*confirmDialog) isDialog()       {}
func (*renameDialog) isDialog()        {}
func (*createChannelDialog) isDialog() {}
func (*browseDialog) isDialog()        {}
func (*addMembersDialog) isDialog()    {}
func (*membersDialog) isDialog()       {}
func (*integrations) isDialog()        {}

// urlName makes a channel's URL name from its display name, as the
// webapp suggests one.
func urlName(display string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(display)) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) || r == '_':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// channelMenu builds the actions on a channel, for the sidebar's menu and,
// with header, the channel header's fuller one.
func (v *View) channelMenu(m *ui.Menu, c *ui.Context, s *app.Server, id store.ChannelID, header bool) {
	a := v.App
	st := s.Store
	ch := st.Channels[id]
	if ch == nil {
		return
	}
	member := st.Members[id]
	dm := ch.Type == store.Direct || ch.Type == store.Group
	if header && !dm {
		if m.Item("View Members").Chosen() {
			v.openMembers(s, id)
		}
		if m.Item("Add Members").Chosen() {
			v.openAddMembers(s, id)
		}
		m.Separator()
		if m.Item("Edit Channel Header").Chosen() {
			v.dialog = &textDialog{title: "Edit Header for " + a.ChannelName(s, ch), label: "Header", value: ch.Header, submit: "Save", lines: true,
				save: func(text string) { a.PatchChannel(s, id, mm.ChannelPatch{Header: &text}, nil) }}
		}
		if m.Item("Edit Channel Purpose").Chosen() {
			v.dialog = &textDialog{title: "Edit Purpose for " + a.ChannelName(s, ch), label: "Purpose", value: ch.Purpose, submit: "Save", lines: true,
				save: func(text string) { a.PatchChannel(s, id, mm.ChannelPatch{Purpose: &text}, nil) }}
		}
		if m.Item("Rename Channel").Chosen() {
			v.dialog = &renameDialog{channel: id, display: ch.DisplayName, urlName: ch.Name}
		}
		if ch.Type == store.Open && ch.Name != "town-square" && m.Item("Convert to Private Channel").Chosen() {
			v.dialog = &confirmDialog{title: "Convert " + a.ChannelName(s, ch) + " to a private channel?", action: "Convert to Private Channel",
				body: "Only its members will see and join it. You cannot make it public again here.", do: func() { a.MakePrivate(s, id) }}
		}
		m.Separator()
	}
	if !header && !st.Unread(id) && m.Item("Mark as Unread").Chosen() {
		a.MarkUnread(s, id)
	}
	fav := a.IsFavorite(s, id)
	label := "Favorite"
	if fav {
		label = "Unfavorite"
	}
	if m.Item(label).Chosen() {
		a.SetFavorite(s, id, !fav)
	}
	muted := member != nil && member.Muted()
	what := "Channel"
	if dm {
		what = "Conversation"
	}
	label = "Mute " + what
	if muted {
		label = "Unmute " + what
	}
	if m.Item(label).Chosen() {
		a.SetMuted(s, id, !muted)
	}
	m.Submenu("Move to…", func(m *ui.Menu) {
		cur := a.Category(s, id)
		for _, cat := range st.Categories[s.Team] {
			if cur != nil && cat.ID == cur.ID {
				continue
			}
			if !dm && cat.Type == store.CategoryDirect || dm && cat.Type == store.CategoryChannels {
				continue
			}
			if m.Item(cat.DisplayName).ID(cat.ID).Chosen() {
				a.MoveToCategory(s, id, cat.ID)
			}
		}
		m.Separator()
		if m.Item("New Category…").Chosen() {
			v.newCategory(s, id)
		}
	})
	m.Separator()
	if m.Item("Copy Link").Chosen() {
		if link := a.ChannelLink(s, id); link != "" {
			c.WriteClipboard(link)
			c.Toast("Link copied")
		}
	}
	if !header && !dm && m.Item("Add Members").Chosen() {
		v.openAddMembers(s, id)
	}
	if dm || ch.Name == "town-square" {
		return
	}
	m.Separator()
	if header && m.Item("Archive Channel").Chosen() {
		v.dialog = &confirmDialog{title: "Archive " + a.ChannelName(s, ch) + "?", action: "Archive",
			body: "Its posts stay readable, but no one can post in it. Members leave it from their sidebars.", do: func() { a.ArchiveChannel(s, id) }}
	}
	if m.Item("Leave Channel").Chosen() {
		if ch.Type == store.Private {
			v.dialog = &confirmDialog{title: "Leave " + a.ChannelName(s, ch) + "?", action: "Leave",
				body: "It is private: you will need an invitation to join it again.", do: func() { a.LeaveChannel(s, id) }}
		} else {
			a.LeaveChannel(s, id)
		}
	}
}

// categoryMenu builds the actions on a sidebar category.
func (v *View) categoryMenu(m *ui.Menu, s *app.Server, key string) {
	a := v.App
	var cat *store.Category
	for _, c := range s.Store.Categories[s.Team] {
		if c.ID == key {
			cat = c
		}
	}
	if cat != nil && cat.Type == store.CategoryCustom {
		if m.Item("Rename Category").Chosen() {
			v.dialog = &textDialog{title: "Rename Category", label: "Name", value: cat.DisplayName, submit: "Rename",
				save: func(name string) { a.RenameCategory(s, cat, name) }}
		}
		if m.Item("Delete Category").Chosen() {
			v.dialog = &confirmDialog{title: "Delete " + cat.DisplayName + "?", action: "Delete",
				body: "Its channels move back to Channels and Direct Messages.", do: func() { a.DeleteCategory(s, cat) }}
		}
		m.Separator()
	}
	if m.Item("Create New Category").Chosen() {
		v.newCategory(s, "")
	}
}

func (v *View) newCategory(s *app.Server, channel store.ChannelID) {
	v.dialog = &textDialog{title: "Create New Category", label: "Name", submit: "Create",
		save: func(name string) { v.App.CreateCategory(s, name, channel) }}
}

func (v *View) openMembers(s *app.Server, id store.ChannelID) {
	d := &membersDialog{channel: id}
	v.dialog = d
	v.App.Members(s, id, func(ms []mm.MemberInfo) { d.members, d.loaded = ms, true })
}

func (v *View) openAddMembers(s *app.Server, id store.ChannelID) {
	d := &addMembersDialog{channel: id, members: map[store.UserID]bool{}}
	v.dialog = d
	v.App.Members(s, id, func(ms []mm.MemberInfo) {
		for _, m := range ms {
			d.members[m.User] = true
		}
	})
}

// dialogs shows the open dialog.
func (v *View) dialogs(c *ui.Context) {
	s := v.App.Current
	if v.dialog == nil {
		return
	}
	if s == nil || !s.Ready {
		v.dialog = nil
		return
	}
	open := true
	ui.Modal(c, &open, func() {
		switch d := v.dialog.(type) {
		case *textDialog:
			v.textDialog(c, d)
		case *confirmDialog:
			v.confirmDialog(c, d)
		case *renameDialog:
			v.renameDialog(c, s, d)
		case *createChannelDialog:
			v.createChannelDialog(c, s, d)
		case *browseDialog:
			v.browseDialog(c, s, d)
		case *addMembersDialog:
			v.addMembersDialog(c, s, d)
		case *membersDialog:
			v.membersDialog(c, s, d)
		case *integrations:
			v.integrationsDialog(c, s, d)
		case *customStatusDialog:
			v.customStatusDialog(c, s, d)
		case *profileDialog:
			v.profileDialog(c, s, d)
		}
	})
	if !open {
		v.dialog = nil
	}
}

func dialogTitle(c *ui.Context, title string) {
	ui.Text(c, title).FontSize(18).Bold().MaxLines(2)
}

// dialogButtons ends a dialog with Cancel and its action.
func (v *View) dialogButtons(c *ui.Context, action string, disabled, submitted bool, do func()) {
	ui.Row(c).Gap(8).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
		if ui.Button(c, "Cancel").Clicked() {
			v.dialog = nil
		}
		if (ui.PrimaryButton(c, action).Disabled(disabled).Clicked() || submitted) && !disabled {
			do()
		}
	})
}

func (v *View) textDialog(c *ui.Context, d *textDialog) {
	ui.Column(c).Width(460).Padding(8).Gap(12).Children(func() {
		dialogTitle(c, d.title)
		submitted := false
		if d.lines {
			ui.TextArea(c.Key("text"), &d.value).Lines(3, 10).Label(d.label).AutoFocus()
		} else {
			submitted = ui.TextInput(c.Key("text"), &d.value).Label(d.label).Placeholder(d.label).AutoFocus().Submitted()
		}
		empty := !d.lines && strings.TrimSpace(d.value) == ""
		v.dialogButtons(c, d.submit, empty, submitted, func() {
			v.dialog = nil
			d.save(strings.TrimSpace(d.value))
		})
	})
}

func (v *View) confirmDialog(c *ui.Context, d *confirmDialog) {
	ui.Column(c).Width(440).Padding(8).Gap(12).Children(func() {
		dialogTitle(c, d.title)
		ui.Text(c, d.body).TextColor(c.Theme().TextMuted)
		v.dialogButtons(c, d.action, false, false, func() {
			v.dialog = nil
			d.do()
		})
	})
}

func (v *View) renameDialog(c *ui.Context, s *app.Server, d *renameDialog) {
	ui.Column(c).Width(460).Padding(8).Gap(8).Children(func() {
		dialogTitle(c, "Rename Channel")
		name := field(c, "display", "Display name", &d.display, "").AutoFocus()
		url := field(c, "url", "URL", &d.urlName, "")
		ui.Text(c, s.URL.Host+"/…/channels/"+d.urlName).FontSize(12).TextColor(c.Theme().TextMuted)
		bad := strings.TrimSpace(d.display) == "" || urlName(d.urlName) != d.urlName
		v.dialogButtons(c, "Rename", bad, name.Submitted() || url.Submitted(), func() {
			v.dialog = nil
			display := strings.TrimSpace(d.display)
			v.App.PatchChannel(s, d.channel, mm.ChannelPatch{DisplayName: &display, Name: &d.urlName}, nil)
		})
	})
}

func (v *View) createChannelDialog(c *ui.Context, s *app.Server, d *createChannelDialog) {
	t := c.Theme()
	ui.Column(c).Width(480).Padding(8).Gap(8).Children(func() {
		dialogTitle(c, "Create a new channel")
		name := field(c, "name", "Name", &d.display, "Enter a name for your channel").AutoFocus()
		if u := urlName(d.display); u != "" {
			ui.Text(c, "URL: "+s.URL.Host+"/…/channels/"+u).FontSize(12).TextColor(t.TextMuted)
		}
		kind := 0
		if d.private {
			kind = 1
		}
		ui.Segmented(c.Key("kind"), &kind, "Public channel", "Private channel").Margin(4, 0)
		d.private = kind == 1
		field(c, "purpose", "Purpose (optional)", &d.purpose, "What is this channel about?")
		if d.err != "" {
			ui.Text(c, d.err).FontSize(13).TextColor(t.Danger)
		}
		bad := d.busy || urlName(d.display) == ""
		v.dialogButtons(c, "Create channel", bad, name.Submitted(), func() {
			typ := store.Open
			if d.private {
				typ = store.Private
			}
			d.busy, d.err = true, ""
			v.App.CreateChannel(s, mm.NewChannel{Name: urlName(d.display), DisplayName: strings.TrimSpace(d.display), Type: typ, Purpose: strings.TrimSpace(d.purpose)}, func(err error) {
				d.busy = false
				if err != nil {
					d.err = err.Error()
					return
				}
				if v.dialog == d {
					v.dialog = nil
				}
			})
		})
	})
}

func (v *View) browseDialog(c *ui.Context, s *app.Server, d *browseDialog) {
	t := c.Theme()
	ui.Column(c).Width(560).Padding(8).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			dialogTitle(c, "Browse channels")
			ui.Spacer(c)
			if ui.Button(c, "Create channel").Clicked() {
				v.dialog = &createChannelDialog{}
			}
		})
		ui.SearchField(c.Key("q"), &d.query).Placeholder("Search channels").AutoFocus()
		ui.Column(c).Height(360).Children(func() {
			if !d.loaded {
				ui.Column(c).Grow(1).Center().Children(func() { ui.Spinner(c) })
				return
			}
			q := strings.ToLower(strings.TrimSpace(d.query))
			var shown []store.Channel
			for _, ch := range d.channels {
				if q == "" || strings.Contains(strings.ToLower(ch.DisplayName), q) || strings.Contains(ch.Name, q) {
					shown = append(shown, ch)
				}
			}
			if len(shown) == 0 {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No other public channels").TextColor(t.TextMuted)
				})
				return
			}
			ui.Scroll(c).Grow(1).Children(func() {
				for _, ch := range shown {
					ui.Row(c.Key(ch.ID)).Padding(8, 4).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
						symbol(c, "globe", 14, t.TextMuted)
						ui.Column(c).Grow(1).Shrink(1).Children(func() {
							ui.Text(c, ch.DisplayName).FontWeight(600).MaxLines(1)
							if ch.Purpose != "" {
								ui.Text(c, ch.Purpose).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
							}
						})
						if ui.Button(c, "Join").Label("Join " + ch.DisplayName).Clicked() {
							v.dialog = nil
							v.App.JoinChannel(s, ch.ID)
						}
					})
				}
			})
		})
	})
}

func (v *View) openBrowse(s *app.Server) {
	d := &browseDialog{}
	v.dialog = d
	v.App.PublicChannels(s, func(chs []store.Channel) {
		slices.SortFunc(chs, func(a, b store.Channel) int {
			return cmp.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
		})
		d.channels, d.loaded = chs, true
	})
}

func (v *View) addMembersDialog(c *ui.Context, s *app.Server, d *addMembersDialog) {
	t := c.Theme()
	a := v.App
	ch := s.Store.Channels[d.channel]
	if ch == nil {
		v.dialog = nil
		return
	}
	ui.Column(c).Width(480).Padding(8).Gap(10).Children(func() {
		dialogTitle(c, "Add people to "+a.ChannelName(s, ch))
		if len(d.picked) > 0 {
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, u := range d.picked {
					chip := ui.ButtonBase(c.Key(u)).Height(26).Padding(0, 6, 0, 3).Radius(13).Background(t.Surface).Label("Remove " + a.UserName(s, u))
					chip.Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							v.avatar(c, s, u, 20)
							ui.Text(c, a.UserName(s, u)).FontSize(13)
							symbol(c, "xmark", 9, t.TextMuted)
						})
					})
					if chip.Clicked() {
						d.picked = slices.DeleteFunc(d.picked, func(x store.UserID) bool { return x == u })
					}
				}
			})
		}
		ui.SearchField(c.Key("q"), &d.query).Placeholder("Search for people").AutoFocus()
		q := strings.ToLower(strings.TrimLeft(strings.TrimSpace(d.query), "@"))
		if q != "" && q != d.asked {
			d.asked = q
			a.SearchUsers(s, q, func(users []store.User) {
				if d.asked != q {
					return
				}
				d.found = d.found[:0]
				for _, u := range users {
					d.found = append(d.found, u.ID)
				}
			})
		}
		ui.Column(c).Height(260).Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				if q == "" {
					ui.Text(c, "Type a name to find people.").TextColor(t.TextMuted).Padding(8, 4)
					return
				}
				for _, id := range d.found {
					u := s.Store.Users[id]
					if u == nil || u.Deleted || slices.Contains(d.picked, id) {
						continue
					}
					in := d.members[id]
					row := ui.ButtonBase(c.Key(id)).FillWidth().Padding(6, 8).Radius(4).Label(u.Username).Disabled(in)
					if row.Hovered() && !in {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
							v.avatar(c, s, id, 24)
							ui.Text(c, u.Username).FontWeight(600)
							if n := u.FullName(); n != "" {
								ui.Text(c, n).FontSize(13).TextColor(t.TextMuted)
							}
							if in {
								ui.Spacer(c)
								ui.Text(c, "Already in channel").FontSize(12).TextColor(t.TextMuted)
							}
						})
					})
					if row.Clicked() && !in {
						d.picked = append(d.picked, id)
						d.query, d.asked = "", ""
					}
				}
			})
		})
		v.dialogButtons(c, "Add", len(d.picked) == 0 || d.busy, false, func() {
			d.busy = true
			n := len(d.picked)
			a.AddMembers(s, d.channel, d.picked, func() {
				if v.dialog == d {
					v.dialog = nil
				}
				v.ShowError(fmt.Sprintf("Added %d to %s", n, a.ChannelName(s, ch)))
			})
		})
	})
}

func (v *View) membersDialog(c *ui.Context, s *app.Server, d *membersDialog) {
	t := c.Theme()
	a := v.App
	ch := s.Store.Channels[d.channel]
	if ch == nil {
		v.dialog = nil
		return
	}
	ui.Column(c).Width(480).Padding(8).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			title := a.ChannelName(s, ch) + " members"
			if d.loaded {
				title = fmt.Sprintf("%s · %d members", a.ChannelName(s, ch), len(d.members))
			}
			dialogTitle(c, title)
			ui.Spacer(c)
			if ui.PrimaryButton(c, "Add Members").Clicked() {
				v.openAddMembers(s, d.channel)
			}
		})
		ui.Column(c).Height(380).Children(func() {
			if !d.loaded {
				ui.Column(c).Grow(1).Center().Children(func() { ui.Spinner(c) })
				return
			}
			members := slices.Clone(d.members)
			slices.SortFunc(members, func(x, y mm.MemberInfo) int {
				if x.Admin != y.Admin {
					if x.Admin {
						return -1
					}
					return 1
				}
				return cmp.Compare(a.UserName(s, x.User), a.UserName(s, y.User))
			})
			ui.Scroll(c).Grow(1).Children(func() {
				for _, m := range members {
					ui.Row(c.Key(m.User)).Padding(6, 4).Gap(10).AlignItems(ui.Center).Children(func() {
						v.avatar(c, s, m.User, 28)
						ui.Column(c).Grow(1).Shrink(1).Children(func() {
							name := a.UserName(s, m.User)
							if m.User == s.Store.Me {
								name += " (you)"
							}
							ui.Text(c, name).FontWeight(600).MaxLines(1)
							if u := s.Store.Users[m.User]; u != nil && u.FullName() != "" {
								ui.Text(c, u.FullName()).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
							}
						})
						if m.Admin {
							ui.Text(c, "Admin").FontSize(12).TextColor(t.TextMuted)
						}
						if m.User != s.Store.Me && ch.Name != "town-square" {
							if ui.Button(c, "Remove").Label("Remove " + a.UserName(s, m.User)).Clicked() {
								user := m.User
								a.RemoveMember(s, d.channel, user, func() {
									d.members = slices.DeleteFunc(d.members, func(x mm.MemberInfo) bool { return x.User == user })
								})
							}
						}
					})
				}
			})
		})
	})
}
