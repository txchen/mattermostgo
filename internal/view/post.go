package view

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// jumpGlow is how long a post jumped to stays highlighted.
const jumpGlow = 3 * time.Second

// groupWindow is how close in time posts by one user join one group.
const groupWindow = 5 * time.Minute

// postKey keeps a pending post's row when the server confirms it.
func postKey(p *store.Post) any {
	if p.PendingID != "" {
		return p.PendingID
	}
	return p.ID
}

// continues reports whether a post joins the group of the one before it.
func continues(prev, p *store.Post) bool {
	return prev != nil && prev.UserID == p.UserID && !prev.IsSystem() && !p.IsSystem() &&
		p.CreateAt-prev.CreateAt < groupWindow.Milliseconds() && prev.RootID == p.RootID &&
		prev.Props.OverrideUsername == p.Props.OverrideUsername && day(prev.CreateAt).Equal(day(p.CreateAt))
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
		var id store.PostID
		if opens {
			id = post.ID
		}
		if compact {
			v.compactRow(c, s, post, grouped, hovered, inThread, id)
		} else {
			ui.Row(c).Padding(top, 20, 4, 20).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Width(36).AlignItems(ui.Center).Children(func() {
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
					v.postBody(c, s, post, id)
					if !inThread && post.RootID == "" && post.ReplyCount > 0 {
						v.replyFooter(c, s, post)
					}
				})
			})
		}
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

// nameColors color the names of compact posts, one for each user.
var nameColors = []ui.Color{ui.Hex("#3d8bfd"), ui.Hex("#20a464"), ui.Hex("#d9822b"), ui.Hex("#b45ad6"), ui.Hex("#d64545"), ui.Hex("#1a9ba3"), ui.Hex("#7a6ff0"), ui.Hex("#c2185b")}

func nameColor(id store.UserID) ui.Color {
	var h uint32
	for i := 0; i < len(id); i++ {
		h = h*31 + uint32(id[i])
	}
	return nameColors[h%uint32(len(nameColors))]
}

// compactRow is a post as the webapp shows it with compact messages: the
// time, then the name in a color of its own, then the text, on one line,
// the name only on the first post of a group.
func (v *View) compactRow(c *ui.Context, s *app.Server, post *store.Post, grouped, hovered, inThread bool, opens store.PostID) {
	t := c.Theme()
	ui.Row(c).Padding(3, 20, 3, 20).Gap(8).AlignItems(ui.Start).Children(func() {
		at := time.UnixMilli(post.CreateAt)
		clock := ui.Text(c, v.clock(at)).FontSize(11).TextColor(t.TextMuted).Width(58).Padding(4, 0, 0, 0).Tooltip(at.Format("Monday, January 2, 2006"))
		if grouped && !hovered {
			clock.Opacity(0)
		}
		name := v.App.UserName(s, post.UserID)
		switch {
		case post.IsSystem():
			name = "System"
		case post.Props.OverrideUsername != "":
			name = post.Props.OverrideUsername
		}
		// The next posts of a group keep the room of the name, unseen, so
		// that their text lines up under the first one.
		who := ui.Text(c, name).FontSize(14).FontWeight(700).TextColor(nameColor(post.UserID)).Padding(1, 0, 0, 0).Shrink(0)
		if grouped {
			who.Opacity(0).AccessHidden(true)
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
			v.postBody(c, s, post, opens)
			if !inThread && post.RootID == "" && post.ReplyCount > 0 {
				v.replyFooter(c, s, post)
			}
		})
	})
}

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
		switch msg := v.prepare(s, post.Message); {
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
