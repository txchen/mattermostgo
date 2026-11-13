package view

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/markdown"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// QuickReactions are the emoji offered for a reaction in one click.
var QuickReactions = []string{"+1", "white_check_mark", "heart", "joy", "tada", "eyes", "pray", "-1"}

// jumpGlow is how long a post jumped to stays highlighted.
const jumpGlow = 3 * time.Second

// groupWindow is how close in time posts by one user join one group.
const groupWindow = 5 * time.Minute

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

// postKey keeps a pending post's row when the server confirms it.
func postKey(p *store.Post) any {
	if p.PendingID != "" {
		return p.PendingID
	}
	return p.ID
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

// continues reports whether a post joins the group of the one before it.
func continues(prev, p *store.Post) bool {
	return prev != nil && prev.UserID == p.UserID && !prev.IsSystem() && !p.IsSystem() &&
		p.CreateAt-prev.CreateAt < groupWindow.Milliseconds() && prev.RootID == p.RootID &&
		prev.Props.OverrideUsername == p.Props.OverrideUsername && day(prev.CreateAt).Equal(day(p.CreateAt))
}

// separator is a line across the list with a label in its middle.
func separator(c *ui.Context, label string, color, line ui.Color) {
	ui.Row(c).Padding(10, 20).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(line)
		ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(color)
		ui.Box(c).Height(1).Grow(1).Background(line)
	})
}

func (v *View) postRow(row ui.ListRow, s *app.Server, order []store.PostID, i int, root store.PostID) {
	c := row.Context
	t := c.Theme()
	p := colors(c)
	inThread := root != ""
	post := s.Store.Posts[order[i]]
	var prev *store.Post
	if i > 0 {
		prev = s.Store.Posts[order[i-1]]
	}
	if inThread && i == 1 {
		n := len(order) - 1
		label := fmt.Sprint(n, " replies")
		if n == 1 {
			label = "1 reply"
		}
		ui.Row(c).Padding(8, 20).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(13).TextColor(t.TextMuted)
			ui.Box(c).Height(1).Grow(1).Background(t.Border)
		})
	} else if prev == nil || !day(prev.CreateAt).Equal(day(post.CreateAt)) {
		separator(c, dayLabel(day(post.CreateAt), day(c.Now().UnixMilli())), t.Text, t.Border)
	}
	if !inThread && s.NewSince > 0 && post.CreateAt > s.NewSince && post.UserID != s.Store.Me && (prev == nil || prev.CreateAt <= s.NewSince) {
		separator(c, "New Messages", p.newLine, p.newLine)
	}
	grouped := continues(prev, post) && !(inThread && i == 1)
	hovered := row.Hovered()
	bg := ui.Transparent
	jump := s.Jump
	highlight := (post.ID == jump.Post || !inThread && post.ID == jump.Root) && c.Now().Sub(jump.At) < jumpGlow
	if highlight {
		c.After(jumpGlow - c.Now().Sub(jump.At))
	}
	switch {
	case highlight:
		bg = p.mentionHighlight
	case v.actionsFor == post.ID:
		bg = t.SurfacePressed
	case hovered:
		bg = t.SurfaceHover
	}
	box := ui.Box(c).FillWidth().Background(bg).ContextMenu(func(m *ui.Menu) { v.postMenu(m, c, s, post, true) })
	// A click on a post opens its thread, as the webapp's "Click to open
	// threads"; what the post holds that is clickable keeps its click.
	opens := !inThread && !post.IsSystem() && !post.Ephemeral && post.State == store.Sent && v.editing != post.ID && s.Store.Pref("display_settings", "click_to_reply") != "false"
	compact := s.Store.Pref("display_settings", "message_display") == "compact"
	if opens {
		if box.Label("Post by " + v.App.UserName(s, post.UserID)).RowClicked() {
			v.App.OpenThread(s, post.ID)
		}
	}
	box.Children(func() {
		top := float32(10)
		if grouped || compact {
			top = 2
		}
		if post.Pinned || s.Store.Saved(post.ID) {
			v.postMarks(c, s, post)
		}
		ui.Row(c).Padding(top, 20, 4, 20).Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Width(36).AlignItems(ui.Center).Children(func() {
				if compact {
					if !grouped || hovered {
						ui.Text(c, v.clock(time.UnixMilli(post.CreateAt))).FontSize(11).TextColor(t.TextMuted).Padding(3, 0)
					}
					return
				}
				switch {
				case !grouped && post.IsSystem():
					ui.Image(c, systemAvatar).Size(36, 36).AccessHidden(true)
				case !grouped:
					v.userButton(c, s, post, 36)
				case hovered:
					ui.Text(c, v.clock(time.UnixMilli(post.CreateAt))).FontSize(10).TextColor(t.TextMuted).Padding(4, 0)
				}
			})
			ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
				if !grouped {
					v.postHeader(c, s, post)
				}
				var id store.PostID
				if opens {
					id = post.ID
				}
				v.postBody(c, s, post, id)
				if !inThread && post.RootID == "" && post.ReplyCount > 0 {
					v.replyFooter(c, s, post)
				}
			})
		})
		if (hovered || v.actionsFor == post.ID) && post.State == store.Sent && v.editing != post.ID && !post.IsSystem() && !post.Ephemeral {
			v.postActions(c, s, post, inThread)
		}
	})
}

// textPress is a press of the primary button on the text of a post, and
// textClick a click there that opens the post's thread once the frame it
// brings is built, unless a link in the text took it.
type textPress struct {
	post store.PostID
	x, y float32
}

type textClick struct {
	server *app.Server
	post   store.PostID
}

// opensThread makes a click on a post's selectable text open the thread
// of the post id, when set, as a click on the rest of the post does. The
// text keeps the pointer, so it is watched rather than asked: a press
// that moves selects and a double click selects a word, without opening
// anything.
func (v *View) opensThread(text ui.Element, s *app.Server, id store.PostID) {
	if id == "" {
		return
	}
	text.ObserveInput(func(ev ui.InputEvent) {
		switch ev.Kind {
		case ui.InputPointerDown:
			v.press = textPress{}
			if ev.Button == 0 && ev.Clicks == 1 && ev.Mods == 0 {
				v.press = textPress{post: id, x: ev.X, y: ev.Y}
			}
		case ui.InputPointerUp:
			p := v.press
			v.press = textPress{}
			if p.post == id && ev.Button == 0 && !ev.Canceled && abs(ev.X-p.x) < 4 && abs(ev.Y-p.y) < 4 {
				v.textClick = textClick{s, id}
			}
		}
	})
}

func abs(f float32) float32 { return max(f, -f) }

// replyFooter sums up a root post's thread under it, as the webapp does
// with Collapsed Reply Threads: who replied, how many replies, and when
// the last came. A click opens the thread in the RHS.
func (v *View) replyFooter(c *ui.Context, s *app.Server, post *store.Post) {
	t := c.Theme()
	p := colors(c)
	btn := ui.ButtonBase(c).AlignSelf(ui.Start).Padding(3, 6).Margin(4, 0, 0, -6).Radius(4).Label("Open thread")
	if btn.Hovered() || s.Thread == post.ID {
		btn.Background(t.SurfaceHover)
	}
	btn.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			people := post.Participants
			if len(people) > 3 {
				people = people[len(people)-3:]
			}
			if len(people) > 0 {
				ui.Row(c).Children(func() {
					for i := len(people) - 1; i >= 0; i-- {
						ui.Box(c.Key(people[i])).Size(22, 22).Radius(11).Background(t.Background).Center().Margin(0, 0, 0, -6*float32(min(i, 1))).Children(func() {
							v.avatar(c, s, people[i], 18)
						})
					}
				})
			}
			symbol(c, "arrowshape.turn.up.left", 12, p.link)
			label := fmt.Sprint(post.ReplyCount, " replies")
			if post.ReplyCount == 1 {
				label = "1 reply"
			}
			ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(p.link)
			if post.LastReplyAt > 0 {
				ui.Text(c, "Last reply "+ago(c.Now(), post.LastReplyAt)).FontSize(12).TextColor(t.TextMuted)
				// The label ticks once a minute, while it shows.
				if c.Now().Sub(time.UnixMilli(post.LastReplyAt)) < 48*time.Hour {
					c.After(time.Minute)
				}
			}
		})
	})
	if btn.Clicked() {
		v.App.OpenThread(s, post.ID)
	}
}

// ago says how long before now a time in milliseconds was, as the
// webapp's thread footers do.
func ago(now time.Time, ms int64) string {
	d := now.Sub(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		if m := int(d.Minutes()); m > 1 {
			return fmt.Sprint(m, " minutes ago")
		}
		return "a minute ago"
	case d < 24*time.Hour:
		if h := int(d.Hours()); h > 1 {
			return fmt.Sprint(h, " hours ago")
		}
		return "an hour ago"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprint(int(d.Hours()/24), " days ago")
	}
}

func (v *View) postHeader(c *ui.Context, s *app.Server, post *store.Post) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
		name := v.App.UserName(s, post.UserID)
		switch {
		case post.IsSystem():
			name = "System"
		case post.Props.OverrideUsername != "":
			name = post.Props.OverrideUsername
		}
		if post.IsSystem() || post.Props.OverrideUsername != "" {
			ui.Text(c, name).FontWeight(700).FontSize(15)
		} else {
			v.userName(c, s, post, name)
		}
		if u := s.Store.Users[post.UserID]; u != nil && !post.IsSystem() && post.Props.OverrideUsername == "" && u.CustomStatus.Shows(c.Now()) {
			ui.Text(c, emojiText(u.CustomStatus.Emoji)).FontSize(14).Tooltip(u.CustomStatus.Text)
		}
		if u := s.Store.Users[post.UserID]; !post.IsSystem() && (u != nil && u.IsBot || post.Props.FromBot) {
			ui.Text(c, "BOT").FontSize(10).FontWeight(700).Padding(1, 4).Radius(3).Background(t.Surface).TextColor(t.TextMuted)
		}
		ts := time.UnixMilli(post.CreateAt)
		if post.Ephemeral {
			ui.Text(c, "(Only visible to you)").FontSize(12).TextColor(t.TextMuted)
		}
		ui.Text(c, v.clock(ts)).FontSize(12).TextColor(t.TextMuted).Tooltip(ts.Format("Monday, January 2, 2006 at ") + v.clock(ts))
	})
}

// plainText reports whether a prepared message shows the same as plain
// text, so that it can skip the Markdown view, which costs more to lay out.
func plainText(msg string) bool {
	if strings.ContainsAny(msg, "*_`[]<>#|~\\&@") || strings.Contains(msg, "http") || strings.Contains(msg, "www.") {
		return false
	}
	for line := range strings.Lines(msg) {
		l := strings.TrimLeft(line, " \t")
		if l == "" || l == "\n" || strings.Trim(l, "-:= \t\n") == "" || strings.HasPrefix(l, "-") || strings.HasPrefix(l, "+") || strings.HasPrefix(l, "=") ||
			(l[0] >= '0' && l[0] <= '9' && strings.ContainsAny(l[:min(len(l), 10)], ".)")) || len(line)-len(l) >= 4 {
			return false
		}
	}
	return true
}

func (v *View) postBody(c *ui.Context, s *app.Server, post *store.Post, opens store.PostID) {
	t := c.Theme()
	if v.editing == post.ID {
		v.editBox(c, s, post)
		return
	}
	body := ui.Column(c).Gap(4)
	if post.State != store.Sent {
		body.Opacity(0.6)
	}
	body.Children(func() {
		switch msg := markdown.Prepare(post.Message); {
		case post.IsSystem():
			ui.Text(c, post.Message).FontSize(14).TextColor(t.TextMuted).Selectable()
		case msg == "":
		case plainText(msg):
			v.opensThread(ui.Text(c, msg).FontSize(15).LineHeight(1.4).Selectable().ContextMenu(v.textMenu(c, s, post)), s, opens)
		default:
			v.opensThread(mdview.View(c, msg, v.markdownOptions(s)).ContextMenu(v.textMenu(c, s, post)), s, opens)
		}
		if post.EditAt > 0 {
			edited := ui.Text(c, "Edited").FontSize(11).TextColor(t.TextMuted).Tooltip(time.UnixMilli(post.EditAt).Format("Edited Monday, January 2 at ") + v.clock(time.UnixMilli(post.EditAt)) + ". Click for the history.")
			if edited.Clicked() {
				v.openEditHistory(s, post)
			}
		}
		for i, a := range post.Attachments {
			ui.Column(c.Key(i)).Children(func() { v.attachment(c, s, post.ID, a, opens) })
		}
		if post.Embed != nil && s.Store.Pref("display_settings", "link_previews") != "false" {
			v.embed(c, s, post.Embed, opens)
		}
		if len(post.Files) > 0 {
			v.files(c, s, post)
		}
		if len(post.Reactions) > 0 {
			v.reactions(c, s, post)
		}
	})
	if post.State == store.Failed {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			symbol(c, "exclamationmark.circle", 12, t.Danger)
			ui.Text(c, "Message failed to send.").FontSize(12).TextColor(t.Danger)
			if ui.Link(c, "Retry", "").FontSize(12).Clicked() {
				v.App.Retry(s, post.ID)
			}
			if ui.Link(c, "Delete", "").FontSize(12).Clicked() {
				v.App.DeletePost(s, post.ID)
			}
		})
	}
}

// markdownOptions are the Markdown view's options for a server: mentions
// and channel links open in the app, other links in the browser.
func (v *View) markdownOptions(s *app.Server) *mdview.Options {
	o := v.md[s.Config.ID]
	if o == nil {
		o = &mdview.Options{Extensions: parser.GFM, Theme: &mdview.Theme{FontSize: 15}, OnLink: func(link string) { v.openLink(s, link) }}
		v.md[s.Config.ID] = o
	}
	return o
}

func (v *View) openLink(s *app.Server, link string) {
	v.textClick = textClick{}
	switch {
	case strings.HasPrefix(link, markdown.MentionScheme):
		name, _ := url.PathUnescape(strings.TrimPrefix(link, markdown.MentionScheme))
		for id, u := range s.Store.Users {
			if u.Username == name && id != s.Store.Me {
				v.App.OpenDM(s, id)
				return
			}
		}
	case strings.HasPrefix(link, markdown.ChannelScheme):
		name, _ := url.PathUnescape(strings.TrimPrefix(link, markdown.ChannelScheme))
		for id, ch := range s.Store.Channels {
			if ch.Name == name && (ch.TeamID == s.Team || ch.TeamID == "") {
				v.App.OpenChannel(s, id)
				return
			}
		}
	default:
		if v.App.OpenPermalink(s, link) {
			return
		}
		if ui.SafeToOpen(link) {
			v.App.OpenURL(link)
		}
	}
}

func (v *View) avatar(c *ui.Context, s *app.Server, id store.UserID, size float32) {
	u := s.Store.Users[id]
	name := "?"
	key := fmt.Sprintf("avatar:%s:%s", s.Config.ID, id)
	if u != nil {
		name = u.Username
		key += fmt.Sprint(":", u.PictureAt)
	}
	var bmp *ui.Bitmap
	if cl := s.Client(); cl != nil && id != "" {
		bmp = v.Media.Get(key, int(size*2), func(ctx context.Context) ([]byte, error) { return cl.ProfileImage(ctx, id) })
	}
	ui.Box(c).Size(size, size).Radius(size / 2).Clip().Shrink(0).Children(func() {
		// The picture rounds itself: a clip alone leaves it square in a
		// drawn menu.
		if bmp != nil {
			ui.Image(c, bmp).Size(size, size).Fit(ui.Cover).Radius(size / 2).Label(name)
			return
		}
		ui.Avatar(c, name, nil).Size(size, size)
	})
}

func (v *View) files(c *ui.Context, s *app.Server, post *store.Post) {
	t := c.Theme()
	cl := s.Client()
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, f := range post.Files {
			ui.Box(c.Key(f.ID)).Children(func() {
				if f.IsImage() && f.HasPreview && cl != nil && f.ID != "" {
					w, h := float32(f.Width), float32(f.Height)
					if w <= 0 || h <= 0 {
						w, h = 320, 240
					}
					scale := min(1, 360/w, 240/h)
					w, h = w*scale, h*scale
					id := f.ID
					bmp := v.Media.Get(fmt.Sprintf("preview:%s:%s", s.Config.ID, id), int(w*2), func(ctx context.Context) ([]byte, error) {
						return cl.File(ctx, id, mm.FilePreview)
					})
					img := ui.ButtonBase(c).Size(w, h).Radius(6).Background(t.Surface).Label(f.Name).Tooltip(f.Name)
					img.Children(func() {
						if bmp != nil {
							ui.Image(c, bmp).Size(w, h).Fit(ui.Cover).Radius(6)
						}
					})
					if img.Clicked() {
						v.preview(s, post.Files, fileIndex(post.Files, f.ID))
					}
					return
				}
				card := ui.ButtonBase(c).Padding(8, 12).Radius(6).BorderWidth(1).BorderColor(t.Border).Label(f.Name)
				card.Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fileGlyph(f)).FontSize(20)
						ui.Column(c).Children(func() {
							ui.Text(c, f.Name).FontSize(13).Bold().MaxLines(1).MaxWidth(240)
							ui.Text(c, strings.ToUpper(f.Extension)+" "+size(f.Size)).FontSize(11).TextColor(t.TextMuted)
						})
					})
				})
				if card.Clicked() {
					v.preview(s, post.Files, fileIndex(post.Files, f.ID))
				}
			})
		}
	})
}

func fileIndex(files []store.FileInfo, id store.FileID) int {
	return max(slices.IndexFunc(files, func(f store.FileInfo) bool { return f.ID == id }), 0)
}

func size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func emojiText(name string) string {
	if e, ok := markdown.Emoji(name); ok {
		return e
	}
	return ":" + name + ":"
}

func (v *View) reactions(c *ui.Context, s *app.Server, post *store.Post) {
	t := c.Theme()
	p := colors(c)
	type group struct {
		name  string
		users []store.UserID
	}
	var groups []*group
	for _, r := range post.Reactions {
		i := slices.IndexFunc(groups, func(g *group) bool { return g.name == r.EmojiName })
		if i < 0 {
			groups = append(groups, &group{name: r.EmojiName})
			i = len(groups) - 1
		}
		groups[i].users = append(groups[i].users, r.UserID)
	}
	ui.Row(c).Wrap().Gap(4).Padding(4, 0, 2, 0).AlignItems(ui.Center).Children(func() {
		for _, g := range groups {
			mine := slices.Contains(g.users, s.Store.Me)
			names := make([]string, len(g.users))
			for i, u := range g.users {
				names[i] = v.App.UserName(s, u)
			}
			chip := ui.ButtonBase(c.Key(g.name)).Height(24).Padding(0, 8).Radius(4).BorderWidth(1).Label(g.name).
				Tooltip(strings.Join(names, ", ") + " reacted with :" + g.name + ":")
			switch {
			case mine:
				chip.Background(p.link.Alpha(0.08)).BorderColor(p.link)
			case chip.Hovered():
				chip.Background(t.SurfaceHover).BorderColor(t.Border)
			default:
				chip.Background(t.Surface).BorderColor(ui.Transparent)
			}
			chip.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					v.emoji(c, s, g.name, 14)
					txt := ui.Text(c, fmt.Sprint(len(g.users))).FontSize(12).FontWeight(600)
					if mine {
						txt.TextColor(p.link)
					} else {
						txt.TextColor(t.TextMuted)
					}
				})
			})
			if chip.Clicked() {
				v.App.ToggleReaction(s, post.ID, g.name)
			}
		}
		add := iconButton(c.Key("add-reaction"), "face.smiling", "Add a reaction", 13, t.TextMuted, t.SurfaceHover)
		if add.Clicked() {
			v.picker = emojiPicker{open: true, post: post.ID}
		}
		if v.picker.open && v.picker.post == post.ID && !v.picker.fromToolbar {
			v.emojiPopover(c, add, s)
		}
	})
}

// emojiPicker is the popover that chooses an emoji for a reaction, or
// for the composer when post is empty.
type emojiPicker struct {
	open        bool
	post        store.PostID
	fromToolbar bool
	// inDialog marks the picker of the custom status dialog.
	inDialog bool
	// tab is the category shown: 0 for the recent emoji.
	tab      int
	composer draftKey
	query    string
	insert   func(name string)
}

// commonEmoji are what the picker shows before anything is typed.
var commonEmoji = []string{
	"+1", "-1", "white_check_mark", "heart", "joy", "smile", "slightly_smiling_face", "wink",
	"tada", "eyes", "pray", "fire", "rocket", "100", "clap", "raised_hands",
	"thinking_face", "sweat_smile", "sob", "scream", "heart_eyes", "sunglasses", "ok_hand", "wave",
	"muscle", "point_up", "star", "sparkles", "zap", "bulb", "warning", "x",
	"heavy_check_mark", "question", "exclamation", "coffee", "beers", "cake", "gift", "trophy",
}

func (v *View) postActions(c *ui.Context, s *app.Server, post *store.Post, inThread bool) {
	t := c.Theme()
	muted := t.TextMuted
	ui.Row(c).Absolute().Top(-14).Right(16).Gap(0).Padding(2).Radius(4).Background(t.Background).BorderWidth(1).BorderColor(t.Border).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.10)).AlignItems(ui.Center).Children(func() {
		for _, e := range QuickReactions[:3] {
			b := ui.ButtonBase(c.Key(e)).Size(32, 28).Radius(4).Center().Label("React with :" + e + ":").Tooltip(":" + e + ":")
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.Children(func() { ui.Text(c, emojiText(e)).FontSize(16) })
			if b.Clicked() {
				v.App.ToggleReaction(s, post.ID, e)
			}
		}
		ui.Box(c).Width(1).Height(18).Background(t.Border).Margin(0, 2)
		react := iconButton(c, "face.smiling", "Add Reaction", 15, muted, t.SurfaceHover)
		if react.Clicked() {
			v.picker = emojiPicker{open: true, post: post.ID, fromToolbar: true}
			v.actionsFor = post.ID
		}
		if v.picker.open && v.picker.post == post.ID && v.picker.fromToolbar {
			v.emojiPopover(c, react, s)
		}
		if !inThread && iconButton(c, "arrowshape.turn.up.left", "Reply", 15, muted, t.SurfaceHover).Clicked() {
			v.App.OpenThread(s, post.ID)
		}
		iconButton(c, "ellipsis", "More actions", 15, muted, t.SurfaceHover).Menu(func(m *ui.Menu) { v.postMenu(m, c, s, post, false) })
	})
	if !v.picker.open {
		v.actionsFor = ""
	}
}

// remindTimes are the choices of the Remind submenu, as the webapp's.
var remindTimes = []clearAfter{dndEnds[1], dndEnds[2], dndEnds[3], dndEnds[4]}

// postMenu builds the actions on a post, for its More button and, with
// context, its context menu, which adds reactions.
func (v *View) postMenu(m *ui.Menu, c *ui.Context, s *app.Server, post *store.Post, context bool) {
	a := v.App
	if post.State != store.Sent || post.Ephemeral {
		return
	}
	if context {
		m.Submenu("Add Reaction", func(m *ui.Menu) {
			for _, e := range QuickReactions {
				if m.Item(emojiText(e) + "  :" + e + ":").Chosen() {
					a.ToggleReaction(s, post.ID, e)
				}
			}
		})
	}
	root := post.ID
	if post.RootID != "" {
		root = post.RootID
	}
	if m.Item("Reply").Chosen() {
		a.OpenThread(s, root)
	}
	if m.Item("Forward…").Chosen() {
		v.dialog = &forwardDialog{post: post.ID}
	}
	if !post.IsSystem() {
		_, following := s.Store.Followed[root]
		label := "Follow Thread"
		if following {
			label = "Unfollow Thread"
		}
		if m.Item(label).Chosen() {
			a.SetFollowing(s, root, !following)
		}
	}
	if m.Item("Mark as Unread").Chosen() {
		a.MarkPostUnread(s, post)
	}
	m.Submenu("Remind", func(m *ui.Menu) {
		for _, r := range remindTimes {
			if m.Item(r.label).Chosen() {
				a.Remind(s, post.ID, r.until(time.Now()))
			}
		}
	})
	saved := s.Store.Saved(post.ID)
	label := "Save"
	if saved {
		label = "Remove from Saved"
	}
	if m.Item(label).Chosen() {
		a.SetSaved(s, post.ID, !saved)
	}
	label = "Pin to Channel"
	if post.Pinned {
		label = "Unpin from Channel"
	}
	if m.Item(label).Chosen() {
		a.SetPinned(s, post.ID, !post.Pinned)
	}
	m.Separator()
	if m.Item("Copy Link").Chosen() {
		c.WriteClipboard(a.Permalink(s, post.ID))
		c.Toast("Link copied")
	}
	if m.Item("Copy Text").Chosen() {
		c.WriteClipboard(post.Message)
	}
	if post.UserID == s.Store.Me && !post.IsSystem() {
		m.Separator()
		if m.Item("Edit").Chosen() {
			v.editing, v.editText = post.ID, post.Message
		}
		if post.EditAt > 0 && m.Item("Edit History").Chosen() {
			v.openEditHistory(s, post)
		}
		if m.Item("Delete").Danger().Chosen() {
			v.dialog = &confirmDialog{title: "Delete this message?", action: "Delete", body: "Its replies go too, and no one can get it back.",
				do: func() { a.DeletePost(s, post.ID) }}
		}
	}
}

// textMenu is the context menu of the text of a post: the commands of
// selected text, then the actions on the post.
func (v *View) textMenu(c *ui.Context, s *app.Server, post *store.Post) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		m.EditItems()
		m.Separator()
		v.postMenu(m, c, s, post, true)
	}
}

// postMarks says above a post that it is pinned or saved, as the webapp.
func (v *View) postMarks(c *ui.Context, s *app.Server, post *store.Post) {
	p := colors(c)
	ui.Row(c).Padding(6, 20, 0, 68).Gap(12).Children(func() {
		if post.Pinned {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				symbol(c, "pin.fill", 10, p.link)
				ui.Text(c, "Pinned").FontSize(11).FontWeight(600).TextColor(p.link)
			})
		}
		if s.Store.Saved(post.ID) {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				symbol(c, "bookmark.fill", 10, p.link)
				ui.Text(c, "Saved").FontSize(11).FontWeight(600).TextColor(p.link)
			})
		}
	})
}

func (v *View) editBox(c *ui.Context, s *app.Server, post *store.Post) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.TextArea(c.Key("edit"), &v.editText).Lines(1, 12).AutoFocus().Label("Edit message").OnTextKey(func(k ui.TextKey) bool {
			switch {
			case k.Key == ui.KeyEnter && k.Mods == 0:
				v.App.EditPost(s, post.ID, k.Text())
				v.editing = ""
				return true
			case k.Key == ui.KeyEscape:
				v.editing = ""
				return true
			}
			return false
		})
		ui.Text(c, "Enter to save, Escape to cancel").FontSize(11).TextColor(t.TextMuted)
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
