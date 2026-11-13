package view

import (
	"embed"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// noSymbol is drawn for a symbol with no picture of its own off macOS.
var noSymbol = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" fill="currentColor"/></svg>`))

// lucide holds the Lucide icons (ISC license, icons/LICENSE) drawn where
// the system has no SF Symbols, as on Linux and Windows.
//
//go:embed icons/*.svg
var lucide embed.FS

// lucideFor names the Lucide icon of each SF Symbol the app draws.
var lucideFor = map[string]string{
	"archivebox": "archive", "arrow.down": "arrow-down", "arrowshape.turn.up.left": "reply", "at": "at-sign",
	"bookmark": "bookmark", "bookmark.fill": "bookmark", "bubble.left.and.bubble.right": "messages-square",
	"chevron.down": "chevron-down", "chevron.left": "chevron-left", "chevron.right": "chevron-right", "doc": "file",
	"ellipsis": "ellipsis", "exclamationmark.circle": "circle-alert", "face.smiling": "smile", "gearshape": "settings",
	"globe": "globe", "info.circle": "info", "link": "link", "lock": "lock", "magnifyingglass": "search",
	"paperclip": "paperclip", "paperplane.fill": "send", "person.2": "users", "pin": "pin", "pin.fill": "pin", "plus": "plus",
	"rectangle.portrait.on.rectangle.portrait": "copy", "wifi.slash": "wifi-off", "xmark.circle.fill": "circle-x", "xmark": "x",
	"person": "user", "rectangle.portrait.and.arrow.right": "log-out", "person.badge.plus": "user-plus",
	"bold": "bold", "italic": "italic", "strikethrough": "strikethrough", "textformat.size": "heading",
	"chevron.left.forwardslash.chevron.right": "code", "text.quote": "quote", "list.bullet": "list", "list.number": "list-ordered",
}

// fallbacks are the parsed Lucide icons, by SF Symbol name.
var fallbacks = map[string]*ui.SVG{}

func fallback(name string) *ui.SVG {
	if svg, ok := fallbacks[name]; ok {
		return svg
	}
	svg := noSymbol
	if file, ok := lucideFor[name]; ok {
		if b, err := lucide.ReadFile("icons/" + file + ".svg"); err == nil {
			if parsed, err := ui.ParseSVG(b); err == nil {
				svg = parsed
			}
		}
	}
	fallbacks[name] = svg
	return svg
}

// systemAvatar heads system posts, as the Mattermost logo does in the webapp.
var systemAvatar = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 36"><circle cx="18" cy="18" r="18" fill="#1c58d9"/><path fill="#fff" d="M23.4 8.6a10.6 10.6 0 1 0 5.4 6.7l-2.6 1a7.9 7.9 0 1 1-4.1-5.2z"/><path fill="#fff" d="M25.6 7.2 19.9 19.6a2.9 2.9 0 0 1-4.6-3.2z"/></svg>`))

// symbol draws an SF Symbol in a color.
func symbol(c *ui.Context, name string, size float32, col ui.Color) ui.Element {
	return sfsymbols.Symbol(c, name).Size(size).Color(col).Fallback(fallback(name)).AccessHidden(true)
}

// iconButton is a square button showing a symbol, named for the pointer's
// tooltip and assistive technology.
func iconButton(c *ui.Context, name, label string, size float32, col, hover ui.Color) ui.Element {
	b := ui.ButtonBase(c).Size(size+12, size+12).Radius(4).Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(hover)
	}
	b.Children(func() { symbol(c, name, size, col) })
	return b
}
