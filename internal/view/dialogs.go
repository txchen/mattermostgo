package view

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// forwardDialog sends a link to a post to another channel, with a comment.
type forwardDialog struct {
	post    store.PostID
	query   string
	to      store.ChannelID
	comment string
}

// historyDialog shows the earlier versions of a post.
type historyDialog struct {
	post     store.PostID
	versions []store.Post
	loaded   bool
}

// peopleDialog starts a DM, or a group message with several people.
type peopleDialog struct {
	query  string
	asked  string
	found  []store.UserID
	picked []store.UserID
}

// inviteDialog invites people to the team: a link to share, emails to
// invite, or users of the server to add.
type inviteDialog struct {
	link   string
	emails string
	people addMembersDialog
}

func (d *forwardDialog) show(v *View, c *ui.Context, s *app.Server) { v.forwardDialog(c, s, d) }
func (d *historyDialog) show(v *View, c *ui.Context, s *app.Server) { v.historyDialog(c, s, d) }
func (d *peopleDialog) show(v *View, c *ui.Context, s *app.Server)  { v.peopleDialog(c, s, d) }
func (d *inviteDialog) show(v *View, c *ui.Context, s *app.Server)  { v.inviteDialog(c, s, d) }

func (v *View) openEditHistory(s *app.Server, post *store.Post) {
	d := &historyDialog{post: post.ID}
	v.dialog = d
	id := post.ID
	app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]store.Post, error) { return c.EditHistory(ctx, id) },
		func(ps []store.Post) { d.versions, d.loaded = ps, true })
}

// channelChoices are the channels a post may go to, matching a query.
func (v *View) channelChoices(s *app.Server, q string) []*store.Channel {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []*store.Channel
	for _, ch := range s.Store.Channels {
		if ch.Deleted || ch.TeamID != "" && ch.TeamID != s.Team {
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(v.App.ChannelName(s, ch)), q) || strings.Contains(ch.Name, q) {
			out = append(out, ch)
		}
	}
	slices.SortFunc(out, func(a, b *store.Channel) int { return cmp.Compare(b.LastPostAt, a.LastPostAt) })
	return out[:min(len(out), 30)]
}

func (v *View) forwardDialog(c *ui.Context, s *app.Server, d *forwardDialog) {
	t := c.Theme()
	a := v.App
	ui.Column(c).Width(500).Padding(8).Gap(10).Children(func() {
		dialogTitle(c, "Forward message")
		if d.to == "" {
			ui.SearchField(c.Key("q"), &d.query).Placeholder("Find a channel or a person").AutoFocus()
			ui.Column(c).Height(260).Children(func() {
				ui.Scroll(c).Grow(1).Children(func() {
					for _, ch := range v.channelChoices(s, d.query) {
						row := ui.ButtonBase(c.Key(ch.ID)).FillWidth().Padding(6, 8).Radius(4).Label(a.ChannelName(s, ch))
						if row.Hovered() {
							row.Background(t.SurfaceHover)
						}
						row.Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								v.channelIcon(c, s, ch, t.TextMuted, colors(c))
								ui.Text(c, a.ChannelName(s, ch))
							})
						})
						if row.Clicked() {
							d.to = ch.ID
						}
					}
				})
			})
		} else {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "To").TextColor(t.TextMuted)
				if ch := s.Store.Channels[d.to]; ch != nil {
					ui.Text(c, a.ChannelName(s, ch)).FontWeight(600)
				}
				if ui.Link(c, "Change", "").Clicked() {
					d.to = ""
				}
			})
			ui.TextArea(c.Key("comment"), &d.comment).Lines(2, 6).Label("Comment").Placeholder("Add a comment (optional)").AutoFocus()
			if p := s.Store.Posts[d.post]; p != nil {
				ui.Column(c).Padding(8, 12).Radius(4).BorderWidth(1).BorderColor(t.Border).Children(func() {
					ui.Text(c, a.UserName(s, p.UserID)).FontWeight(600).FontSize(13)
					ui.Text(c, p.Message).FontSize(13).MaxLines(4).TextColor(t.TextMuted)
				})
			}
		}
		v.dialogButtons(c, "Forward", d.to == "", false, func() {
			v.dialog = nil
			a.ForwardPost(s, d.post, d.to, d.comment)
		})
	})
}

func (v *View) historyDialog(c *ui.Context, s *app.Server, d *historyDialog) {
	t := c.Theme()
	post := s.Store.Posts[d.post]
	ui.Column(c).Width(520).Padding(8).Gap(10).Children(func() {
		dialogTitle(c, "Edit History")
		ui.Column(c).MaxHeight(440).Children(func() {
			ui.Scroll(c).Children(func() {
				ui.Column(c).Gap(8).Children(func() {
					if post != nil {
						v.historyVersion(c, s, "Current version", post)
					}
					if !d.loaded {
						ui.Spinner(c)
						return
					}
					for _, p := range d.versions {
						at := time.UnixMilli(cmp.Or(p.EditAt, p.CreateAt))
						v.historyVersion(c.Key(p.ID), s, at.Format("Mon Jan 2, ")+v.clock(at), &p)
					}
					if len(d.versions) == 0 {
						ui.Text(c, "No earlier versions").TextColor(t.TextMuted)
					}
				})
			})
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			if ui.Button(c, "Close").Clicked() {
				v.dialog = nil
			}
		})
	})
}

func (v *View) historyVersion(c *ui.Context, s *app.Server, label string, p *store.Post) {
	t := c.Theme()
	ui.Column(c).Padding(8, 12).Gap(4).Radius(4).BorderWidth(1).BorderColor(t.Border).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(t.TextMuted)
		v.postBody(c, s, p, "")
	})
}

func (v *View) peopleDialog(c *ui.Context, s *app.Server, d *peopleDialog) {
	t := c.Theme()
	a := v.App
	ui.Column(c).Width(500).Padding(8).Gap(10).Children(func() {
		dialogTitle(c, "Direct Messages")
		ui.Text(c, "Pick one person for a direct message, or up to seven for a group message.").FontSize(13).TextColor(t.TextMuted)
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
		if q != d.asked {
			d.asked = q
			if q != "" {
				a.SearchUsers(s, q, func(users []store.User) {
					if d.asked == q {
						d.found = d.found[:0]
						for _, u := range users {
							d.found = append(d.found, u.ID)
						}
					}
				})
			}
		}
		ui.Column(c).Height(260).Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				ids := d.found
				if q == "" {
					ids = v.recentPartners(s)
				}
				for _, id := range ids {
					u := s.Store.Users[id]
					if u == nil || u.Deleted || id == s.Store.Me || slices.Contains(d.picked, id) {
						continue
					}
					row := ui.ButtonBase(c.Key(id)).FillWidth().Padding(6, 8).Radius(4).Label(u.Username)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
							v.avatar(c, s, id, 24)
							ui.Text(c, u.Username).FontWeight(600)
							if n := u.FullName(); n != "" {
								ui.Text(c, n).FontSize(13).TextColor(t.TextMuted)
							}
						})
					})
					if row.Clicked() && len(d.picked) < 7 {
						d.picked = append(d.picked, id)
						d.query = ""
					}
				}
			})
		})
		action := "Go"
		v.dialogButtons(c, action, len(d.picked) == 0, false, func() {
			v.dialog = nil
			if len(d.picked) == 1 {
				a.OpenDM(s, d.picked[0])
				return
			}
			a.OpenGroup(s, d.picked)
		})
	})
}

// recentPartners are the people of the recent DMs, newest first.
func (v *View) recentPartners(s *app.Server) []store.UserID {
	var dms []*store.Channel
	for _, ch := range s.Store.Channels {
		if ch.Type == store.Direct {
			dms = append(dms, ch)
		}
	}
	slices.SortFunc(dms, func(a, b *store.Channel) int { return cmp.Compare(b.LastPostAt, a.LastPostAt) })
	var out []store.UserID
	for _, ch := range dms[:min(len(dms), 20)] {
		out = append(out, ch.DMPartner(s.Store.Me))
	}
	return out
}

func (v *View) openInvite(s *app.Server) {
	d := &inviteDialog{people: addMembersDialog{members: map[store.UserID]bool{}}}
	v.dialog = d
	team := s.Team
	app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (string, error) {
		id, err := c.TeamInvite(ctx, team)
		if err != nil || id == "" {
			return "", err
		}
		return c.InviteLink(id), nil
	}, func(link string) { d.link = link })
}

func (v *View) inviteDialog(c *ui.Context, s *app.Server, d *inviteDialog) {
	t := c.Theme()
	a := v.App
	team := s.Store.Teams[s.Team]
	name := ""
	if team != nil {
		name = team.DisplayName
	}
	ui.Column(c).Width(520).Padding(8).Gap(10).Children(func() {
		dialogTitle(c, "Invite people to "+name)
		ui.Text(c, "SHARE A LINK").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			link := d.link
			if link == "" {
				link = "Only team admins can share the link."
			}
			ui.Text(c, link).FontSize(12).Font("monospace").Grow(1).Shrink(1).MaxLines(1).Selectable()
			if ui.Button(c, "Copy Link").Disabled(d.link == "").Clicked() {
				c.WriteClipboard(d.link)
				c.Toast("Link copied")
			}
		})
		ui.Text(c, "ADD PEOPLE OF THIS SERVER").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Margin(8, 0, 0, 0)
		p := &d.people
		ui.SearchField(c.Key("q"), &p.query).Placeholder("Search for people")
		q := strings.ToLower(strings.TrimSpace(p.query))
		if q != p.asked {
			p.asked = q
			if q != "" {
				a.SearchUsers(s, q, func(users []store.User) {
					if p.asked == q {
						p.found = p.found[:0]
						for _, u := range users {
							p.found = append(p.found, u.ID)
						}
					}
				})
			}
		}
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, u := range p.picked {
				if ui.Button(c.Key(u), a.UserName(s, u)+" ×").Clicked() {
					p.picked = slices.DeleteFunc(p.picked, func(x store.UserID) bool { return x == u })
				}
			}
		})
		if q != "" {
			ui.Column(c).MaxHeight(150).Children(func() {
				ui.Scroll(c).Children(func() {
					for _, id := range p.found {
						u := s.Store.Users[id]
						if u == nil || slices.Contains(p.picked, id) {
							continue
						}
						if ui.Button(c.Key(id), u.Username).Clicked() {
							p.picked = append(p.picked, id)
							p.query = ""
						}
					}
				})
			})
		}
		ui.Text(c, "INVITE BY EMAIL").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Margin(8, 0, 0, 0)
		ui.TextInput(c.Key("emails"), &d.emails).Placeholder("name@example.com, another@example.com").Label("Emails")
		emails := lines(d.emails)
		v.dialogButtons(c, "Invite", len(p.picked) == 0 && len(emails) == 0, false, func() {
			teamID, users := s.Team, p.picked
			v.dialog = nil
			app.Call(a, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
				if len(users) > 0 {
					if err := c.AddTeamMembers(ctx, teamID, users); err != nil {
						return struct{}{}, err
					}
				}
				if len(emails) > 0 {
					return struct{}{}, c.InviteEmails(ctx, teamID, emails)
				}
				return struct{}{}, nil
			}, func(struct{}) { v.ShowError("Invitations sent") })
		})
	})
}

// dialog is the one dialog the window shows over the main view, if any;
// show builds it, inside the modal.
type dialog interface {
	show(v *View, c *ui.Context, s *app.Server)
}

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
		v.dialog.show(v, c, s)
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
