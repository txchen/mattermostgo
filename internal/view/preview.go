package view

import (
	"context"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// previewDialog shows the files of a post one at a time, as the webapp's
// file preview: images drawn large, text and code shown in the window,
// other files in Quick Look.
type previewDialog struct {
	files []store.FileInfo
	index int
	// text holds the text files loaded, by ID.
	text map[store.FileID]string
}

func (d *previewDialog) show(v *View, c *ui.Context, s *app.Server) { v.previewDialog(c, s, d) }

// textExts are the files shown as text, by extension.
var textExts = map[string]string{
	"txt": "", "md": "markdown", "log": "", "csv": "", "json": "json", "yaml": "yaml", "yml": "yaml", "toml": "toml",
	"go": "go", "py": "python", "js": "javascript", "ts": "typescript", "tsx": "tsx", "jsx": "jsx", "rs": "rust",
	"java": "java", "kt": "kotlin", "swift": "swift", "c": "c", "h": "c", "cpp": "cpp", "cc": "cpp", "rb": "ruby",
	"sh": "bash", "zsh": "bash", "sql": "sql", "html": "html", "css": "css", "xml": "xml", "diff": "diff", "patch": "diff",
	"ini": "ini", "conf": "", "env": "", "tf": "hcl", "lua": "lua", "php": "php", "proto": "protobuf",
}

const maxTextPreview = 512 << 10

func textLang(f store.FileInfo) (string, bool) {
	ext := strings.ToLower(f.Extension)
	if ext == "" {
		ext = strings.TrimPrefix(strings.ToLower(path.Ext(f.Name)), ".")
	}
	lang, ok := textExts[ext]
	return lang, ok && f.Size <= maxTextPreview
}

// fileGlyph is the picture of a file card, by its kind.
func fileGlyph(f store.FileInfo) string {
	switch {
	case f.IsImage():
		return "🖼️"
	case strings.HasPrefix(f.MimeType, "video/"):
		return "🎬"
	case strings.HasPrefix(f.MimeType, "audio/"):
		return "🎵"
	case f.MimeType == "application/pdf":
		return "📕"
	case strings.Contains(f.MimeType, "zip") || strings.Contains(f.MimeType, "compressed"):
		return "🗜️"
	}
	if _, ok := textLang(f); ok {
		return "📝"
	}
	return "📄"
}

// preview opens files in the preview, at one of them. A file the window
// cannot show goes to Quick Look.
func (v *View) preview(s *app.Server, files []store.FileInfo, i int) {
	f := files[i]
	if _, text := textLang(f); !f.IsImage() && !text {
		v.App.QuickLookFile(s, f)
		return
	}
	v.dialog = &previewDialog{files: files, index: i, text: map[store.FileID]string{}}
}

func (v *View) previewDialog(c *ui.Context, s *app.Server, d *previewDialog) {
	t := c.Theme()
	w, h := c.Size()
	f := d.files[d.index]
	cl := s.Client()
	if c.Shortcut(0, ui.KeyLeft) && d.index > 0 {
		d.index--
	}
	if c.Shortcut(0, ui.KeyRight) && d.index < len(d.files)-1 {
		d.index++
	}
	ui.Column(c).Width(min(w-80, 1100)).Height(min(h-80, 820)).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fileGlyph(f)).FontSize(20)
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				ui.Text(c, f.Name).FontWeight(700).MaxLines(1)
				detail := size(f.Size)
				if len(d.files) > 1 {
					detail = fmt.Sprintf("%d of %d · %s", d.index+1, len(d.files), detail)
				}
				ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted)
			})
			if len(d.files) > 1 {
				if iconButton(c, "chevron.left", "Previous file", 13, t.TextMuted, t.SurfaceHover).Disabled(d.index == 0).Clicked() && d.index > 0 {
					d.index--
				}
				if iconButton(c, "chevron.right", "Next file", 13, t.TextMuted, t.SurfaceHover).Disabled(d.index == len(d.files)-1).Clicked() && d.index < len(d.files)-1 {
					d.index++
				}
			}
			if ui.Button(c, "Open").Tooltip("Open in its app").Clicked() {
				v.App.OpenFile(s, f)
			}
			if ui.Button(c, "Download").Clicked() {
				v.App.Download(s, f)
			}
			if iconButton(c, "xmark", "Close preview", 14, t.TextMuted, t.SurfaceHover).Clicked() {
				v.dialog = nil
			}
		})
		ui.Box(c).Grow(1).FillWidth().Radius(6).Background(t.Surface).Clip().Center().Children(func() {
			switch lang, text := textLang(f); {
			case f.IsImage() && cl != nil:
				bw, bh := min(w-120, 1060), min(h-180, 720)
				iw, ih := float32(f.Width), float32(f.Height)
				if iw <= 0 || ih <= 0 {
					iw, ih = bw, bh
				}
				scale := min(1, bw/iw, bh/ih)
				id := f.ID
				bmp := v.Media.Get(fmt.Sprintf("full:%s:%s", s.Config.ID, id), int(max(iw, ih)*scale*2), func(ctx context.Context) ([]byte, error) {
					return cl.File(ctx, id, mm.FileFull)
				})
				if bmp == nil {
					ui.Spinner(c)
					return
				}
				ui.Image(c, bmp).Size(iw*scale, ih*scale).Fit(ui.Contain)
			case text:
				body, ok := d.text[f.ID]
				if !ok {
					d.text[f.ID] = ""
					id := f.ID
					app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]byte, error) { return c.File(ctx, id, mm.FileFull) }, func(b []byte) {
						if !utf8.Valid(b) {
							d.text[id] = "This file is not text."
							return
						}
						d.text[id] = string(b)
					})
				}
				if body == "" {
					ui.Spinner(c)
					return
				}
				ui.Scroll(c).Fill().Children(func() {
					if lang == "markdown" {
						mdview.View(c, body, v.markdownOptions(s)).Padding(16)
						return
					}
					fence := "```"
					for strings.Contains(body, fence) {
						fence += "`"
					}
					mdview.View(c, fence+lang+"\n"+body+"\n"+fence, v.markdownOptions(s)).Padding(8)
				})
			}
		})
	})
}
