package view

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/markdown"
	"github.com/txchen/mattermostgo/internal/store"
)

// webImage draws an image from the web, scaled to fit w by h and keeping
// its shape when its size is known.
func (v *View) webImage(c *ui.Context, s *app.Server, img store.Image, w, h float32) ui.Element {
	if img.Width > 0 && img.Height > 0 {
		scale := min(1, w/float32(img.Width), h/float32(img.Height))
		w, h = float32(img.Width)*scale, float32(img.Height)*scale
	}
	var bmp *ui.Bitmap
	if cl := s.Client(); cl != nil && img.URL != "" {
		u := img.URL
		bmp = v.Media.Get(fmt.Sprintf("web:%s:%s", s.Config.ID, u), int(max(w, h)*2), func(ctx context.Context) ([]byte, error) {
			return cl.WebImage(ctx, u)
		})
	}
	box := ui.Box(c).Size(w, h).Radius(4).Clip().Background(c.Theme().Surface).Shrink(0)
	box.Children(func() {
		if bmp != nil {
			ui.Image(c, bmp).Size(w, h).Fit(ui.Cover)
		}
	})
	return box
}

// embed shows the preview of a post's link, as the webapp's OpenGraph
// card, or the image itself for a link to one.
func (v *View) embed(c *ui.Context, s *app.Server, e *store.Embed) {
	t := c.Theme()
	p := colors(c)
	if e.ImageOnly {
		img := ui.ButtonBase(c).AlignSelf(ui.Start).Label("Open image").Tooltip(e.URL)
		img.Children(func() { v.webImage(c, s, *e.Image, 480, 360) })
		if img.Clicked() {
			v.openLink(s, e.URL)
		}
		return
	}
	large := e.Image != nil && e.Image.Width >= 400 && e.Image.Width >= e.Image.Height
	ui.Row(c).FillWidth().MaxWidth(560).Margin(4, 0, 0, 0).Children(func() {
		ui.Box(c).Width(4).FillHeight().Radius(2).Background(t.Border)
		ui.Column(c).Grow(1).Shrink(1).Padding(8, 12).Gap(4).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
					if e.SiteName != "" {
						ui.Text(c, e.SiteName).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
					}
					if e.Title != "" {
						title := ui.Text(c, e.Title).FontSize(14).FontWeight(600).TextColor(p.link).MaxLines(2).Tooltip(e.URL)
						if title.Clicked() {
							v.openLink(s, e.URL)
						}
					}
					if e.Description != "" {
						ui.Text(c, e.Description).FontSize(13).LineHeight(1.4).MaxLines(3).Selectable()
					}
				})
				if e.Image != nil && !large {
					v.webImage(c, s, *e.Image, 80, 80)
				}
			})
			if large {
				img := ui.ButtonBase(c).AlignSelf(ui.Start).Margin(4, 0, 0, 0).Label("Open link").Tooltip(e.URL)
				img.Children(func() { v.webImage(c, s, *e.Image, 500, 260) })
				if img.Clicked() {
					v.openLink(s, e.URL)
				}
			}
		})
	})
}

// attachmentColor reads an attachment's color: a hex color, or one of
// Slack's names.
func attachmentColor(c *ui.Context, name string) ui.Color {
	switch name {
	case "good":
		return ui.Hex("#00c100")
	case "warning":
		return ui.Hex("#dede01")
	case "danger":
		return ui.Hex("#e40303")
	}
	if strings.HasPrefix(name, "#") && (len(name) == 4 || len(name) == 7) {
		return ui.Hex(name)
	}
	return c.Theme().Border
}

// attachment shows a message attachment as the webapp does: a colored
// bar, the author, title, text, fields, image and footer.
func (v *View) attachment(c *ui.Context, s *app.Server, a store.Attachment) {
	t := c.Theme()
	p := colors(c)
	md := v.markdownOptions(s)
	if a.Pretext != "" {
		mdview.View(c, markdown.Prepare(a.Pretext), md).AlignSelf(ui.Start)
	}
	ui.Row(c).FillWidth().MaxWidth(640).Margin(4, 0, 0, 0).Radius(4).BorderWidth(1).BorderColor(t.Border).Clip().Children(func() {
		ui.Box(c).Width(4).FillHeight().Background(attachmentColor(c, a.Color))
		ui.Column(c).Grow(1).Shrink(1).Padding(10, 12).Gap(6).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(6).Children(func() {
					if a.AuthorName != "" {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							if a.AuthorIcon != "" {
								v.webImage(c, s, store.Image{URL: a.AuthorIcon}, 16, 16).Radius(8)
							}
							name := ui.Text(c, a.AuthorName).FontSize(13).FontWeight(600)
							if a.AuthorLink != "" {
								name.TextColor(p.link)
								if name.Clicked() {
									v.openLink(s, a.AuthorLink)
								}
							}
						})
					}
					if a.Title != "" {
						title := ui.Text(c, a.Title).FontSize(15).FontWeight(700)
						if a.TitleLink != "" {
							title.TextColor(p.link).Tooltip(a.TitleLink)
							if title.Clicked() {
								v.openLink(s, a.TitleLink)
							}
						}
					}
					if a.Text != "" {
						mdview.View(c, markdown.Prepare(a.Text), md).AlignSelf(ui.Start)
					} else if a.Title == "" && a.Fallback != "" && len(a.Fields) == 0 {
						ui.Text(c, a.Fallback).FontSize(14).Selectable()
					}
				})
				if a.ThumbURL != "" {
					v.webImage(c, s, store.Image{URL: a.ThumbURL}, 75, 75)
				}
			})
			if len(a.Fields) > 0 {
				v.attachmentFields(c, s, a.Fields)
			}
			if a.ImageURL != "" {
				img := ui.ButtonBase(c).AlignSelf(ui.Start).Label("Open image").Tooltip(a.ImageURL)
				img.Children(func() { v.webImage(c, s, store.Image{URL: a.ImageURL}, 400, 300) })
				if img.Clicked() {
					v.openLink(s, a.ImageURL)
				}
			}
			if a.Footer != "" {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if a.FooterIcon != "" {
						v.webImage(c, s, store.Image{URL: a.FooterIcon}, 16, 16).Radius(8)
					}
					ui.Text(c, a.Footer).FontSize(12).TextColor(t.TextMuted)
				})
			}
		})
	})
}

// attachmentFields lays fields out as the webapp: a short field shares
// its line with the next short one, a long field takes the whole line.
func (v *View) attachmentFields(c *ui.Context, s *app.Server, fields []store.AttachmentField) {
	md := v.markdownOptions(s)
	field := func(f store.AttachmentField) {
		ui.Column(c).Grow(1).Shrink(1).Basis(0).Gap(2).Children(func() {
			if f.Title != "" {
				ui.Text(c, f.Title).FontSize(13).FontWeight(700)
			}
			if f.Value != "" {
				mdview.View(c, markdown.Prepare(f.Value), md).AlignSelf(ui.Start)
			}
		})
	}
	ui.Column(c).Gap(8).Children(func() {
		for i := 0; i < len(fields); i++ {
			f := fields[i]
			ui.Row(c.Key(i)).Gap(16).Children(func() {
				field(f)
				if f.Short && i+1 < len(fields) && fields[i+1].Short {
					i++
					field(fields[i])
				}
			})
		}
	})
}
