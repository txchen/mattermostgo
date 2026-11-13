package view

import (
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// noSymbol is drawn where the system has no SF Symbols, as off macOS.
var noSymbol = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" fill="currentColor"/></svg>`))

// systemAvatar heads system posts, as the Mattermost logo does in the webapp.
var systemAvatar = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 36"><circle cx="18" cy="18" r="18" fill="#1c58d9"/><path fill="#fff" d="M23.4 8.6a10.6 10.6 0 1 0 5.4 6.7l-2.6 1a7.9 7.9 0 1 1-4.1-5.2z"/><path fill="#fff" d="M25.6 7.2 19.9 19.6a2.9 2.9 0 0 1-4.6-3.2z"/></svg>`))

// symbol draws an SF Symbol in a color.
func symbol(c *ui.Context, name string, size float32, col ui.Color) ui.Element {
	return sfsymbols.Symbol(c, name).Size(size).Color(col).Fallback(noSymbol).AccessHidden(true)
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
