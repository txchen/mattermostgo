package view

import (
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// noSymbol is drawn where the system has no SF Symbols, as off macOS.
var noSymbol = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" fill="currentColor"/></svg>`))

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
