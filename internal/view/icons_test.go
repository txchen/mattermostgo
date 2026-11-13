package view

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Every symbol the app draws has a Lucide icon for Linux and Windows,
// which parses.
func TestSymbolFallbacks(t *testing.T) {
	for name := range lucideFor {
		if fallback(name) == noSymbol {
			t.Errorf("no icon for %s", name)
		}
	}
	dir := os.Getenv("MM_FRAMES")
	if dir == "" {
		return
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Wrap().Gap(12).Padding(16).Children(func() {
			for name := range lucideFor {
				ui.Icon(c.Key(name), fallback(name)).Size(20, 20)
			}
		})
	}, 600, 200)
	tt.Frame()
	f, _ := os.Create(filepath.Join(dir, "fallback-icons.png"))
	defer f.Close()
	png.Encode(f, tt.Image())
}
