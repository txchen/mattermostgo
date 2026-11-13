package view

import "github.com/egoist/mygo/ui"

// look is a theme of the window: the colors of the widgets and those of
// the parts the widgets do not draw, as the sidebar.
type look struct {
	id, name string
	dark     bool
	// lightChrome marks a light top bar and sidebar.
	lightChrome bool
	// bg, surface, border, text, muted and accent color the widgets.
	bg, surface, border, text, muted, accent, danger ui.Color
	pal                                              palette
}

// looks are the themes Settings offers. The dark ones are soft, with
// gray-blue grounds and muted text, as One Dark, rather than black and
// white.
var looks = []look{
	{
		id: "denim", name: "Denim", bg: ui.Hex("#ffffff"), surface: ui.Hex("#f4f5f7"), border: ui.RGBA(63, 67, 80, 0.16),
		text: ui.Hex("#3f4350"), muted: ui.RGBA(63, 67, 80, 0.64), accent: ui.Hex("#1c58d9"), danger: ui.Hex("#d24b4e"),
		pal: palette{
			topBar: ui.Hex("#14213e"), teamBar: ui.Hex("#14213e"), sidebar: ui.Hex("#1e325c"),
			sidebarText: ui.RGBA(255, 255, 255, 0.64), sidebarTextStrong: ui.Hex("#ffffff"),
			sidebarHover: ui.RGBA(255, 255, 255, 0.08), sidebarActive: ui.RGBA(255, 255, 255, 0.16),
			mentionBg: ui.Hex("#ffffff"), mentionText: ui.Hex("#1e325c"), newLine: ui.Hex("#cc8f00"),
			online: ui.Hex("#3db887"), away: ui.Hex("#ffbc1f"), dnd: ui.Hex("#d24b4e"), offline: ui.RGBA(255, 255, 255, 0.32),
			mentionHighlight: ui.RGBA(255, 188, 31, 0.16),
			cardBorder:       ui.RGBA(63, 67, 80, 0.16), link: ui.Hex("#386fe5"), buttonBg: ui.Hex("#1c58d9"),
		},
	},
	{
		id: "quartz", name: "Quartz", lightChrome: true, bg: ui.Hex("#ffffff"), surface: ui.Hex("#f4f4f6"), border: ui.RGBA(61, 60, 64, 0.14),
		text: ui.Hex("#3d3c40"), muted: ui.RGBA(61, 60, 64, 0.62), accent: ui.Hex("#166de0"), danger: ui.Hex("#d24b4e"),
		pal: palette{
			topBar: ui.Hex("#eceef1"), teamBar: ui.Hex("#e3e5e9"), sidebar: ui.Hex("#f4f5f7"),
			sidebarText: ui.RGBA(61, 60, 64, 0.72), sidebarTextStrong: ui.Hex("#2d3039"),
			sidebarHover: ui.RGBA(61, 60, 64, 0.06), sidebarActive: ui.RGBA(22, 109, 224, 0.10),
			mentionBg: ui.Hex("#166de0"), mentionText: ui.Hex("#ffffff"), newLine: ui.Hex("#d24b4e"),
			online: ui.Hex("#3db887"), away: ui.Hex("#ffbc1f"), dnd: ui.Hex("#d24b4e"), offline: ui.RGBA(61, 60, 64, 0.32),
			mentionHighlight: ui.RGBA(255, 188, 31, 0.18),
			cardBorder:       ui.RGBA(61, 60, 64, 0.12), link: ui.Hex("#166de0"), buttonBg: ui.Hex("#166de0"),
		},
	},
	{
		id: "onedark", name: "One Dark", dark: true, bg: ui.Hex("#282c34"), surface: ui.Hex("#2f343e"), border: ui.Hex("#3b4048"),
		text: ui.Hex("#abb2bf"), muted: ui.Hex("#7f848e"), accent: ui.Hex("#61afef"), danger: ui.Hex("#e06c75"),
		pal: palette{
			topBar: ui.Hex("#1e2227"), teamBar: ui.Hex("#1e2227"), sidebar: ui.Hex("#21252b"),
			sidebarText: ui.Hex("#8b919c"), sidebarTextStrong: ui.Hex("#d7dae0"),
			sidebarHover: ui.RGBA(171, 178, 191, 0.08), sidebarActive: ui.RGBA(97, 175, 239, 0.16),
			mentionBg: ui.Hex("#61afef"), mentionText: ui.Hex("#21252b"), newLine: ui.Hex("#e06c75"),
			online: ui.Hex("#98c379"), away: ui.Hex("#e5c07b"), dnd: ui.Hex("#e06c75"), offline: ui.Hex("#5c6370"),
			mentionHighlight: ui.RGBA(229, 192, 123, 0.12),
			cardBorder:       ui.Hex("#3b4048"), link: ui.Hex("#61afef"), buttonBg: ui.Hex("#4d8fd1"),
		},
	},
	{
		id: "nord", name: "Nord", dark: true, bg: ui.Hex("#2e3440"), surface: ui.Hex("#353c4a"), border: ui.Hex("#434c5e"),
		text: ui.Hex("#d8dee9"), muted: ui.Hex("#8a93a5"), accent: ui.Hex("#88c0d0"), danger: ui.Hex("#bf616a"),
		pal: palette{
			topBar: ui.Hex("#242933"), teamBar: ui.Hex("#242933"), sidebar: ui.Hex("#2a303b"),
			sidebarText: ui.Hex("#99a2b4"), sidebarTextStrong: ui.Hex("#e5e9f0"),
			sidebarHover: ui.RGBA(216, 222, 233, 0.07), sidebarActive: ui.RGBA(136, 192, 208, 0.16),
			mentionBg: ui.Hex("#88c0d0"), mentionText: ui.Hex("#2e3440"), newLine: ui.Hex("#d08770"),
			online: ui.Hex("#a3be8c"), away: ui.Hex("#ebcb8b"), dnd: ui.Hex("#bf616a"), offline: ui.Hex("#616e88"),
			mentionHighlight: ui.RGBA(235, 203, 139, 0.12),
			cardBorder:       ui.Hex("#434c5e"), link: ui.Hex("#88c0d0"), buttonBg: ui.Hex("#5e81ac"),
		},
	},
	{
		id: "dusk", name: "Dusk", dark: true, bg: ui.Hex("#2b2d31"), surface: ui.Hex("#313338"), border: ui.Hex("#3f4147"),
		text: ui.Hex("#c9ccd1"), muted: ui.Hex("#8e9297"), accent: ui.Hex("#7aa2f7"), danger: ui.Hex("#e5747a"),
		pal: palette{
			topBar: ui.Hex("#1f2023"), teamBar: ui.Hex("#1f2023"), sidebar: ui.Hex("#232428"),
			sidebarText: ui.Hex("#949ba4"), sidebarTextStrong: ui.Hex("#dbdee1"),
			sidebarHover: ui.RGBA(201, 204, 209, 0.07), sidebarActive: ui.RGBA(122, 162, 247, 0.16),
			mentionBg: ui.Hex("#7aa2f7"), mentionText: ui.Hex("#1f2023"), newLine: ui.Hex("#e5747a"),
			online: ui.Hex("#6cc28a"), away: ui.Hex("#e5c07b"), dnd: ui.Hex("#e5747a"), offline: ui.Hex("#5c5f66"),
			mentionHighlight: ui.RGBA(229, 192, 123, 0.12),
			cardBorder:       ui.Hex("#3f4147"), link: ui.Hex("#7aa2f7"), buttonBg: ui.Hex("#5b7fd6"),
		},
	},
}

// activeLook is the theme the window builds with, set at each frame.
var activeLook = &looks[0]

// lookFor returns the theme named, or for "" the one following the
// system: Denim when light, One Dark when dark.
func lookFor(id string, systemDark bool) *look {
	if id == "" {
		id = "denim"
		if systemDark {
			id = "onedark"
		}
	}
	for i := range looks {
		if looks[i].id == id {
			return &looks[i]
		}
	}
	return &looks[0]
}

// setTheme themes the window as the user chose.
func (v *View) setTheme(c *ui.Context) {
	l := lookFor(v.App.Theme, c.Theme().Dark)
	activeLook = l
	base := ui.LightTheme()
	if l.dark {
		base = ui.DarkTheme()
	}
	t := *base.WithAccent(l.accent)
	t.Background, t.Surface, t.Border, t.Text, t.TextMuted, t.Danger = l.bg, l.surface, l.border, l.text, l.muted, l.danger
	hover, pressed := ui.Hex("#000000"), ui.Hex("#000000")
	if l.dark {
		hover, pressed = ui.Hex("#ffffff"), ui.Hex("#ffffff")
	}
	t.SurfaceHover = l.surface.Mix(hover, 0.05)
	t.SurfacePressed = l.surface.Mix(pressed, 0.09)
	t.Selection = l.accent.Alpha(0.28)
	if l.dark {
		t.Inverse, t.InverseText = l.pal.topBar, l.pal.sidebarTextStrong
	}
	c.SetTheme(&t)
}
