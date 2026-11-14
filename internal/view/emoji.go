package view

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/markdown"
	"github.com/txchen/mattermostgo/internal/store"
)

// emoji draws an emoji by its name: the system one, or a custom emoji of
// the server as its picture, or its name when there is neither.
func (v *View) emoji(c *ui.Context, s *app.Server, name string, size float32) {
	if e, ok := markdown.Emoji(name); ok {
		ui.Text(c, e).FontSize(size)
		return
	}
	if _, ok := v.App.CustomEmojis(s)[name]; !ok {
		ui.Text(c, ":"+name+":").FontSize(size * 0.6)
		return
	}
	px := size * 1.25
	bmp := v.emojiPicture(s, name)
	ui.Box(c).Size(px, px).Children(func() {
		if bmp != nil {
			ui.Image(c, bmp).Size(px, px).Fit(ui.Contain).Label(":" + name + ":")
		}
	})
}

// skinTone is the tone the user picks emoji in, 0 for none, kept as the
// webapp keeps it.
func skinTone(s *app.Server) int {
	codes := []string{"default", "1F3FB", "1F3FC", "1F3FD", "1F3FE", "1F3FF"}
	return max(slices.Index(codes, s.Store.Pref("emoji", "emoji_skintone")), 0)
}

func setSkinTone(a *app.App, s *app.Server, tone int) {
	codes := []string{"default", "1F3FB", "1F3FC", "1F3FD", "1F3FE", "1F3FF"}
	a.SetPrefs(s, store.Preference{Category: "emoji", Name: "emoji_skintone", Value: codes[tone]})
}

type recentEmoji struct {
	Name       string `json:"name"`
	UsageCount int    `json:"usageCount"`
}

// recentEmojis are the emoji the user picked lately, most used first, as
// the webapp keeps them in a preference.
func recentEmojis(s *app.Server) []recentEmoji {
	var rs []recentEmoji
	_ = json.Unmarshal([]byte(s.Store.Pref("recent_emojis", string(s.Store.Me))), &rs)
	return rs
}

func useEmoji(a *app.App, s *app.Server, name string) {
	rs := recentEmojis(s)
	i := slices.IndexFunc(rs, func(r recentEmoji) bool { return r.Name == name })
	if i < 0 {
		rs = append(rs, recentEmoji{Name: name})
		i = len(rs) - 1
	}
	rs[i].UsageCount++
	sort.SliceStable(rs, func(x, y int) bool { return rs[x].UsageCount > rs[y].UsageCount })
	rs = rs[:min(len(rs), 27)]
	b, _ := json.Marshal(rs)
	a.SetPrefs(s, store.Preference{Category: "recent_emojis", Name: string(s.Store.Me), Value: string(b)})
}

// emojiPopover is the emoji picker: recent emoji, the system ones by
// category in a skin tone, the custom ones of the server, and a search.
func (v *View) emojiPopover(c *ui.Context, anchor ui.Element, s *app.Server) {
	open := v.picker.open
	ui.Popover(c, anchor, &open, func() {
		t := c.Theme()
		a := v.App
		tone := skinTone(s)
		custom := a.CustomEmojis(s)
		cats := markdown.EmojiCategories()
		ui.Column(c).Width(372).Padding(8).Gap(8).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.SearchField(c.Key("emoji-q"), &v.picker.query).Placeholder("Search emoji").Label("Search emoji").AutoFocus().Grow(1)
				hands := []string{"✋", "✋🏻", "✋🏼", "✋🏽", "✋🏾", "✋🏿"}
				tones := ui.ButtonBase(c).Size(28, 28).Radius(4).Center().Label("Skin tone")
				tones.Children(func() { ui.Text(c, hands[tone]).FontSize(18) }).Menu(func(m *ui.Menu) {
					for i, h := range hands {
						if m.Item(h).Checked(i == tone).Chosen() {
							setSkinTone(a, s, i)
						}
					}
				})
			})
			q := strings.TrimSpace(strings.ToLower(strings.Trim(v.picker.query, ":")))
			var names []string
			title := ""
			switch {
			case q != "":
				title = "Search results"
				for _, n := range markdown.EmojiNames(q, 200) {
					if !strings.Contains(n, "_skin_tone") {
						names = append(names, n)
					}
				}
				for n := range custom {
					if strings.Contains(n, q) {
						names = append(names, n)
					}
				}
				names = names[:min(len(names), 80)]
			default:
				ui.Row(c).Gap(2).Children(func() {
					tab := func(i int, label, glyph string) {
						b := ui.ButtonBase(c.Key(fmt.Sprint("tab", i))).Size(32, 28).Radius(4).Center().Label(label).Tooltip(label)
						if v.picker.tab == i {
							b.Background(t.Selection)
						} else if b.Hovered() {
							b.Background(t.SurfaceHover)
						}
						b.Children(func() { ui.Text(c, glyph).FontSize(16) })
						if b.Clicked() {
							v.picker.tab = i
						}
					}
					tab(0, "Recent", "🕘")
					glyphs := []string{"😀", "👋", "🐶", "🍔", "⚽", "✈️", "💡", "❤️", "🏁"}
					for i, cat := range cats {
						tab(i+1, cat.Name, glyphs[min(i, len(glyphs)-1)])
					}
					if len(custom) > 0 {
						tab(len(cats)+1, "Custom", "⭐")
					}
				})
				switch tab := v.picker.tab; {
				case tab == 0:
					title = "Recently used"
					for _, r := range recentEmojis(s) {
						names = append(names, r.Name)
					}
					if len(names) == 0 {
						names = commonEmoji
					}
				case tab <= len(cats):
					title, names = cats[tab-1].Name, cats[tab-1].Emojis
				default:
					title = "Custom"
					for n := range custom {
						names = append(names, n)
					}
					slices.Sort(names)
				}
			}
			ui.Text(c, strings.ToUpper(title)).FontSize(11).FontWeight(600).TextColor(t.TextMuted)
			if len(names) == 0 {
				ui.Text(c, "No emoji found").TextColor(t.TextMuted).Padding(8)
			}
			ui.Column(c).Height(260).Children(func() {
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Row(c).Wrap().Gap(2).Children(func() {
						for _, name := range names {
							name := markdown.WithSkinTone(name, tone)
							b := ui.ButtonBase(c.Key(name)).Size(38, 38).Radius(4).Center().Label(":" + name + ":").Tooltip(":" + name + ":")
							if b.Hovered() {
								b.Background(t.SurfaceHover)
							}
							b.Children(func() { v.emoji(c, s, name, 22) })
							if b.Clicked() {
								pick := v.picker
								v.picker = emojiPicker{}
								useEmoji(a, s, name)
								if pick.insert != nil {
									pick.insert(name)
								} else {
									a.ToggleReaction(s, pick.post, name)
								}
							}
						}
					})
				})
			})
		})
	})
	if !open {
		v.picker = emojiPicker{}
	}
}

// prepare readies a message for the Markdown view, with the custom emoji
// of the server it knows.
func (v *View) prepare(s *app.Server, msg string) string {
	if !strings.Contains(msg, ":") {
		return markdown.Prepare(msg)
	}
	custom := v.App.CustomEmojis(s)
	return markdown.PrepareWith(msg, func(name string) bool {
		_, ok := custom[name]
		// Only an emoji whose picture loaded becomes a link: the view
		// asks for the pictures of a paragraph as its text changes, so
		// the text changes as the picture comes.
		return ok && v.emojiPicture(s, name) != nil
	})
}

// emojiPixels is the size custom emoji decode at, for text up to 24 DIPs
// on a display of twice the density.
const emojiPixels = 64

// emojiPicture returns the picture of a custom emoji, or nil while it loads.
func (v *View) emojiPicture(s *app.Server, name string) *ui.Bitmap {
	id := v.App.CustomEmojis(s)[name]
	cl := s.Client()
	if id == "" || cl == nil {
		return nil
	}
	return v.Media.Get(fmt.Sprintf("emoji:%s:%s", s.Config.ID, id), emojiPixels, func(ctx context.Context) ([]byte, error) { return cl.EmojiImage(ctx, id) })
}

// customEmoji draws a custom emoji of a message, in the line of its text.
func (v *View) customEmoji(s *app.Server, dest string, size float32) (mdview.InlineObject, bool) {
	name, ok := strings.CutPrefix(dest, markdown.EmojiScheme)
	bmp := v.emojiPicture(s, name)
	if !ok || bmp == nil {
		return mdview.InlineObject{}, false
	}
	px := size * 1.3
	return mdview.InlineObject{Width: px, Height: size, Depth: px - size, Draw: func(p *ui.Painter, r ui.Rect) {
		p.Image(bmp, r, ui.Contain)
	}}, true
}

func emojiText(name string) string {
	if e, ok := markdown.Emoji(name); ok {
		return e
	}
	return ":" + name + ":"
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
