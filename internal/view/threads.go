package view

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// threadsView lists the threads the user follows in the team, as the
// webapp's Threads, newest reply first; one opens in the RHS.
func (v *View) threadsView(c *ui.Context, s *app.Server) {
	t := c.Theme()
	p := colors(c)
	a := v.App
	var threads []*store.ThreadState
	for _, th := range s.Store.Followed {
		if th.Team == s.Team && s.Store.Posts[th.Root] != nil {
			threads = append(threads, th)
		}
	}
	slices.SortFunc(threads, func(x, y *store.ThreadState) int { return cmp.Compare(y.LastReplyAt, x.LastReplyAt) })
	ui.Column(c).Grow(1).FillHeight().Background(t.Background).Children(func() {
		ui.Row(c).Height(62).Padding(0, 16, 0, 20).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			symbol(c, "bubble.left.and.bubble.right", 15, t.TextMuted)
			ui.Text(c, "Followed threads").FontWeight(700).FontSize(16).Role(ui.RoleHeading)
			ui.Spacer(c)
			if n, _ := s.Store.UnreadThreads(s.Team); n > 0 && ui.Button(c, "Mark all as read").Clicked() {
				for _, th := range threads {
					a.MarkThreadRead(s, th.Root)
				}
			}
		})
		if len(threads) == 0 {
			ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
				ui.Text(c, "No followed threads yet").FontSize(16).FontWeight(600)
				ui.Text(c, "Threads you reply in, or follow from their menu, show here.").TextColor(t.TextMuted)
			})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			for _, th := range threads {
				root := s.Store.Posts[th.Root]
				row := ui.ButtonBase(c.Key(th.Root)).FillWidth().Padding(12, 20).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Label("Thread by " + a.UserName(s, root.UserID))
				switch {
				case s.Thread == th.Root:
					row.Background(p.mentionHighlight)
				case row.Hovered():
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
						ui.Box(c).Width(8).Padding(8, 0).Children(func() {
							if th.UnreadReplies > 0 {
								ui.Box(c).Size(8, 8).Radius(4).Background(p.link)
							}
						})
						ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								name := ui.Text(c, a.UserName(s, root.UserID)).FontWeight(600)
								if th.UnreadReplies > 0 {
									name.FontWeight(700)
								}
								if ch := s.Store.Channels[root.ChannelID]; ch != nil {
									ui.Text(c, a.ChannelName(s, ch)).FontSize(11).Padding(1, 6).Radius(4).Background(t.Surface).TextColor(t.TextMuted)
								}
								ui.Spacer(c)
								if th.UnreadMentions > 0 {
									ui.Text(c, count(th.UnreadMentions)).FontSize(11).Bold().Padding(1, 6).Radius(9).Background(t.Accent).TextColor(t.AccentText)
								}
								ui.Text(c, ago(c.Now(), th.LastReplyAt)).FontSize(12).TextColor(t.TextMuted)
							})
							msg := strings.TrimSpace(plainMarkdown.Replace(root.Message))
							if msg == "" && len(root.Files) > 0 {
								msg = "Attachment"
							}
							ui.Text(c, msg).MaxLines(2).TextColor(t.Text)
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								for i, u := range th.Participants[:min(len(th.Participants), 4)] {
									ui.Box(c.Key(i)).Margin(0, 0, 0, -6*float32(min(i, 1))).Children(func() { v.avatar(c, s, u, 20) })
								}
								label := fmt.Sprint(th.ReplyCount, " replies")
								if th.ReplyCount == 1 {
									label = "1 reply"
								}
								if th.UnreadReplies > 0 {
									label += fmt.Sprint(" · ", th.UnreadReplies, " new")
								}
								ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(t.TextMuted)
							})
						})
					})
				}).ContextMenu(func(m *ui.Menu) {
					if m.Item("Unfollow Thread").Chosen() {
						a.SetFollowing(s, th.Root, false)
					}
					if m.Item("Mark as Read").Chosen() {
						a.MarkThreadRead(s, th.Root)
					}
					if m.Item("Open in Channel").Chosen() {
						a.OpenPost(s, th.Root)
					}
					if m.Item("Copy Link").Chosen() {
						c.WriteClipboard(a.Permalink(s, th.Root))
					}
				})
				if row.Clicked() {
					a.OpenThread(s, th.Root)
				}
			}
		})
	})
}

// plainMarkdown takes the marks of emphasis out of a message, for a
// one-line summary of it.
var plainMarkdown = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "")

// popOut opens a channel, or a thread with root set, in a window of its
// own, as the webapp's Open in new window.
func (v *View) popOut(s *app.Server, ch store.ChannelID, root store.PostID) {
	if v.Hooks.OpenWindow == nil {
		return
	}
	title := "Thread"
	if c := s.Store.Channels[ch]; c != nil {
		if root == "" {
			title = v.App.ChannelName(s, c)
		} else {
			title += " in " + v.App.ChannelName(s, c)
		}
	}
	if root != "" {
		v.App.OpenThreadData(s, root)
	} else {
		v.App.LoadChannel(s, ch)
	}
	v.Hooks.OpenWindow(title, func(c *ui.Context) {
		v.window = "pop:" + string(ch) + string(root) + ":"
		defer func() { v.window = "" }()
		v.setTheme(c)
		t := c.Theme()
		if _, in := s.Session.(app.LoggedIn); !in || s.Store.Channels[ch] == nil {
			ui.Column(c).Fill().Center().Children(func() { ui.Text(c, "This conversation is closed.").TextColor(t.TextMuted) })
			return
		}
		ui.Column(c).Fill().Background(t.Background).Children(func() {
			ui.Row(c).Height(max(c.TitleBar().Height, 28)).FillWidth().DragWindow()
			if root == "" {
				v.channelHeader(c, s, s.Store.Channels[ch])
			}
			v.postList(c, s, ch, root)
			v.composer(c, s, ch, root)
		})
	})
}
