package view

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// QuickReactions are the emoji offered for a reaction in one click.
var QuickReactions = []string{"+1", "white_check_mark", "heart", "joy", "tada", "eyes", "pray", "-1"}

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
