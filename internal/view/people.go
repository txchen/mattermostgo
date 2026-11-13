package view

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// clock formats a time of day as the user chose: 24 hours, or 12.
func (v *View) clock(t time.Time) string {
	if s := v.App.Current; s != nil && s.Store.Pref("display_settings", "use_military_time") == "false" {
		return t.Format("3:04 PM")
	}
	return t.Format("15:04")
}

// userButton is the avatar of a post's author, which opens their profile.
func (v *View) userButton(c *ui.Context, s *app.Server, post *store.Post, size float32) {
	key := string(post.ID) + "/avatar"
	b := ui.ButtonBase(c).Size(size, size).Radius(size / 2).Label("Profile of " + v.App.UserName(s, post.UserID))
	b.Children(func() {
		if post.Props.OverrideIconURL != "" {
			v.webImage(c, s, store.Image{URL: post.Props.OverrideIconURL}, size, size).Radius(size / 2)
		} else {
			v.avatar(c, s, post.UserID, size)
		}
	})
	v.profilePopover(c, s, b, key, post.UserID)
}

// userName is the name heading a post, which opens the profile too.
func (v *View) userName(c *ui.Context, s *app.Server, post *store.Post, name string) {
	key := string(post.ID) + "/name"
	b := ui.ButtonBase(c).Label(name)
	b.Children(func() {
		txt := ui.Text(c, name).FontWeight(700).FontSize(15)
		if b.Hovered() {
			txt.Underline()
		}
	})
	v.profilePopover(c, s, b, key, post.UserID)
}

// profilePopover shows a user's card under an element clicked, as the
// webapp's profile popover: picture, names, position, status, and a
// button to message them.
func (v *View) profilePopover(c *ui.Context, s *app.Server, anchor ui.Element, key string, id store.UserID) {
	if anchor.Clicked() {
		v.profileKey = key
		v.App.AskUser(s, id)
	}
	open := v.profileKey == key
	if !open {
		return
	}
	ui.Popover(c, anchor, &open, func() {
		t := c.Theme()
		p := colors(c)
		u := s.Store.Users[id]
		ui.Column(c).Width(300).Padding(16).Gap(8).Children(func() {
			if u == nil {
				ui.Spinner(c)
				return
			}
			ui.Box(c).Size(88, 88).AlignSelf(ui.Center).Children(func() {
				v.avatar(c, s, id, 88)
				ui.Box(c).Absolute().Right(2).Bottom(2).Size(20, 20).Radius(10).Background(t.Background).Center().Children(func() {
					ui.Box(c).Size(14, 14).Radius(7).Background(statusColor(s.Store.Statuses[id], p))
				})
			})
			ui.Column(c).AlignItems(ui.Center).Gap(2).Children(func() {
				if n := u.FullName(); n != "" {
					ui.Text(c, n).FontSize(16).FontWeight(700)
				}
				ui.Text(c, "@"+u.Username).TextColor(t.TextMuted)
				if u.Nickname != "" {
					ui.Text(c, u.Nickname).FontSize(13).TextColor(t.TextMuted)
				}
				if u.Position != "" {
					ui.Text(c, u.Position).FontSize(13)
				}
				if u.IsBot {
					ui.Text(c, "BOT").FontSize(10).FontWeight(700).Padding(1, 4).Radius(3).Background(t.Surface).TextColor(t.TextMuted)
				}
			})
			if cs := u.CustomStatus; cs.Shows(c.Now()) {
				ui.Row(c).Gap(6).AlignSelf(ui.Center).Children(func() {
					ui.Text(c, emojiText(cs.Emoji))
					ui.Text(c, cs.Text).FontSize(13)
				})
			}
			if u.Email != "" {
				ui.Text(c, u.Email).FontSize(12).TextColor(t.TextMuted).AlignSelf(ui.Center).Selectable()
			}
			if id != s.Store.Me && ui.PrimaryButton(c, "Message").FillWidth().Clicked() {
				v.profileKey = ""
				v.App.OpenDM(s, id)
			}
		})
	})
	if !open {
		v.profileKey = ""
	}
}
