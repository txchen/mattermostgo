// Package view draws the client's window with mygo's native UI.
package view

import (
	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/config"
	"github.com/txchen/mattermostgo/internal/media"
	"github.com/txchen/mattermostgo/internal/store"
)

// View is the window's state that is not the app's: forms, scroll
// positions and drafts.
type View struct {
	App   *app.App
	Media *media.Cache
	Hooks Hooks

	adding                    bool
	addName, addURL, addError string
	addBusy                   bool
	editServer                *app.Server
	editName                  string
	loginID, password, mfa    string

	lists  map[listKey]*ui.ListState
	drafts map[draftKey]*string
	// complete is a completion chosen with the keyboard, put in the
	// composer as the next frame builds it.
	complete    map[draftKey]completion
	sent        map[draftKey]string
	inserts     map[draftKey]string
	suggest     map[draftKey]*suggestions
	attachments map[draftKey][]attachment

	editing store.PostID
	// actionsFor is the post whose actions stay shown while a popover of
	// theirs is open; moreFor the post whose "more" popover is open.
	actionsFor, moreFor store.PostID
	picker              emojiPicker
	editText            string
	deleting            store.PostID
	switcher            switcher
	collapsed           map[string]bool
	toasts              []string
	md                  map[config.ServerID]*mdview.Options
	rhsSize             float32
	dialog              dialog
	// serverMenu and meMenu are the top bar's popovers.
	serverMenu, meMenu, plusMenu bool
}

type listKey struct {
	server config.ServerID
	id     string
}

type draftKey struct {
	server  config.ServerID
	channel store.ChannelID
	root    store.PostID
}

func New(a *app.App, m *media.Cache) *View {
	v := &View{App: a, Media: m, lists: map[listKey]*ui.ListState{}, drafts: map[draftKey]*string{},
		complete: map[draftKey]completion{}, sent: map[draftKey]string{}, inserts: map[draftKey]string{}, attachments: map[draftKey][]attachment{}, suggest: map[draftKey]*suggestions{}, collapsed: map[string]bool{}, rhsSize: 62}
	v.md = map[config.ServerID]*mdview.Options{}
	return v
}

// ShowError shows a message at the bottom of the window.
func (v *View) ShowError(msg string) { v.toasts = append(v.toasts, msg) }

func (v *View) list(s *app.Server, id string) *ui.ListState {
	k := listKey{s.Config.ID, id}
	l := v.lists[k]
	if l == nil {
		l = &ui.ListState{}
		v.lists[k] = l
	}
	return l
}

// palette holds the colors of the Mattermost webapp's default themes,
// Denim (light) and Onyx (dark), for the parts the theme does not cover.
type palette struct {
	topBar, teamBar, sidebar, sidebarText, sidebarTextStrong, sidebarHover, sidebarActive ui.Color
	mentionBg, mentionText, newLine, online, away, dnd, offline, mentionHighlight         ui.Color
	cardBorder, link, buttonBg                                                            ui.Color
}

func colors(c *ui.Context) palette {
	if c.Theme().Dark {
		return palette{
			topBar: ui.Hex("#191b1f"), teamBar: ui.Hex("#191b1f"), sidebar: ui.Hex("#202228"),
			sidebarText: ui.RGBA(221, 223, 228, 0.64), sidebarTextStrong: ui.Hex("#dddfe4"),
			sidebarHover: ui.RGBA(255, 255, 255, 0.08), sidebarActive: ui.RGBA(74, 124, 232, 0.24),
			mentionBg: ui.Hex("#4a7ce8"), mentionText: ui.Hex("#ffffff"), newLine: ui.Hex("#de5e5e"),
			online: ui.Hex("#3db887"), away: ui.Hex("#ffbc1f"), dnd: ui.Hex("#d24b4e"), offline: ui.RGBA(221, 223, 228, 0.32),
			mentionHighlight: ui.RGBA(74, 124, 232, 0.16),
			cardBorder:       ui.RGBA(221, 223, 228, 0.12), link: ui.Hex("#5d89ea"), buttonBg: ui.Hex("#4a7ce8"),
		}
	}
	return palette{
		topBar: ui.Hex("#14213e"), teamBar: ui.Hex("#14213e"), sidebar: ui.Hex("#1e325c"),
		sidebarText: ui.RGBA(255, 255, 255, 0.64), sidebarTextStrong: ui.Hex("#ffffff"),
		sidebarHover: ui.RGBA(255, 255, 255, 0.08), sidebarActive: ui.RGBA(255, 255, 255, 0.16),
		mentionBg: ui.Hex("#ffffff"), mentionText: ui.Hex("#1e325c"), newLine: ui.Hex("#cc8f00"),
		online: ui.Hex("#3db887"), away: ui.Hex("#ffbc1f"), dnd: ui.Hex("#d24b4e"), offline: ui.RGBA(255, 255, 255, 0.32),
		mentionHighlight: ui.RGBA(255, 188, 31, 0.16),
		cardBorder:       ui.RGBA(63, 67, 80, 0.16), link: ui.Hex("#386fe5"), buttonBg: ui.Hex("#1c58d9"),
	}
}

// Build is the window's view.
func (v *View) Build(c *ui.Context) {
	for _, msg := range v.toasts {
		c.Toast(msg)
	}
	v.toasts = v.toasts[:0]
	a := v.App
	ui.Column(c).Fill().Children(func() {
		if len(a.Servers) == 0 {
			ui.Row(c).FillWidth().Height(max(c.TitleBar().Height, 28)).Background(c.Theme().Surface).DragWindow()
			v.welcome(c)
			return
		}
		v.topBar(c)
		s := a.Current
		ui.Column(c.Key(s.Config.ID)).Grow(1).Children(func() {
			switch sess := s.Session.(type) {
			case app.LoggedIn:
				if !s.Ready {
					ui.Column(c).Grow(1).Center().Gap(12).Children(func() {
						ui.Spinner(c)
						ui.Text(c, "Loading "+s.Config.Name+"…").TextColor(c.Theme().TextMuted)
					})
					return
				}
				v.main(c, s)
			default:
				v.login(c, s, sess)
			}
		})
	})
	v.addServerDialog(c)
	v.editServerDialog(c)
	v.switcherDialog(c)
	v.dialogs(c)
}

func (v *View) welcome(c *ui.Context) {
	authPage(c, func() {
		ui.Text(c, "Mattermost Go").FontSize(26).FontWeight(700)
		ui.Text(c, "Connect to your Mattermost server to get started.").FontSize(14).TextColor(c.Theme().TextMuted).Margin(0, 0, 8, 0)
		v.serverForm(c)
	})
}

// authPage centers a card on the window, as the webapp's login pages.
func authPage(c *ui.Context, fn func()) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Center().Background(t.Surface).Children(func() {
		ui.Column(c).Width(440).Padding(40).Gap(12).Radius(8).Background(t.Background).BorderWidth(1).BorderColor(t.Border).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.08)).Children(fn)
	})
}

// field is a labeled text input of a form.
func field(c *ui.Context, key, label string, value *string, placeholder string) ui.Element {
	ui.Text(c, label).FontSize(13).FontWeight(600).Margin(4, 0, 0, 0)
	return ui.TextInput(c.Key(key), value).Placeholder(placeholder).Label(label).Height(40).FontSize(15)
}

// primary is a full-width button in the webapp's button color.
func primary(c *ui.Context, label string, disabled bool) ui.Element {
	p := colors(c)
	b := ui.ButtonBase(c).FillWidth().Height(40).Radius(4).Center().Label(label).Disabled(disabled).Margin(8, 0, 0, 0)
	bg := p.buttonBg
	if disabled {
		bg = bg.Alpha(0.4)
	} else if b.Hovered() {
		bg = bg.Mix(ui.Hex("#000000"), 0.1)
	}
	b.Background(bg).Children(func() {
		ui.Text(c, label).FontSize(15).FontWeight(600).TextColor(ui.Hex("#ffffff"))
	})
	return b
}

// serverForm asks for a server's address and name.
func (v *View) serverForm(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		url := field(c, "url", "Server URL", &v.addURL, "https://chat.example.com").AutoFocus()
		name := field(c, "name", "Display name", &v.addName, "Optional, as Work or Community")
		if v.addError != "" {
			ui.Text(c, v.addError).TextColor(t.Danger).FontSize(13)
		}
		add := primary(c, "Add Server", v.addBusy || v.addURL == "")
		if (add.Clicked() || url.Submitted() || name.Submitted()) && !v.addBusy && v.addURL != "" {
			v.addBusy, v.addError = true, ""
			v.App.AddServer(v.addName, v.addURL, func(err error) {
				v.addBusy = false
				if err != nil {
					v.addError = err.Error()
					return
				}
				v.adding, v.addName, v.addURL = false, "", ""
			})
		}
		if v.addBusy {
			ui.Spinner(c).AlignSelf(ui.Center)
		}
		if v.adding && ui.Button(c, "Cancel").AlignSelf(ui.Center).Clicked() {
			v.adding, v.addError = false, ""
		}
	})
}

func (v *View) addServerDialog(c *ui.Context) {
	ui.Modal(c, &v.adding, func() {
		ui.Column(c).Padding(8).Gap(12).Children(func() {
			ui.Text(c, "Add Server").FontSize(18).Bold()
			v.serverForm(c)
		})
	})
}

func (v *View) editServerDialog(c *ui.Context) {
	open := v.editServer != nil
	ui.Modal(c, &open, func() {
		s := v.editServer
		ui.Column(c).Padding(8).Gap(12).Width(360).Children(func() {
			ui.Text(c, "Edit Server").FontSize(18).Bold()
			ui.Text(c, s.URL.String()).TextColor(c.Theme().TextMuted)
			in := ui.TextInput(c, &v.editName).Label("Display name").AutoFocus()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					v.editServer = nil
				}
				if ui.PrimaryButton(c, "Save").Clicked() || in.Submitted() {
					v.App.EditServer(s, v.editName)
					v.editServer = nil
				}
			})
		})
	})
	if !open {
		v.editServer = nil
	}
}

func (v *View) login(c *ui.Context, s *app.Server, sess app.Session) {
	t := c.Theme()
	busy := false
	if li, ok := sess.(app.LoggingIn); ok {
		busy = true
		sess = li.Prev
	}
	authPage(c, func() {
		ui.Text(c, "Log in to "+s.Config.Name).FontSize(24).FontWeight(700)
		ui.Text(c, s.URL.String()).FontSize(14).TextColor(t.TextMuted).Margin(0, 0, 8, 0)
		errMsg := ""
		switch sess := sess.(type) {
		case app.NeedsMFA:
			errMsg = sess.Err
			ui.Text(c, "Enter the code from your authenticator app.").FontSize(14).TextColor(t.TextMuted)
			in := field(c, "mfa", "MFA code", &v.mfa, "123456").AutoFocus()
			if (primary(c, "Verify", busy || v.mfa == "").Clicked() || in.Submitted()) && v.mfa != "" && !busy {
				v.App.Login(s, sess.LoginID, sess.Password, v.mfa)
				v.mfa = ""
			}
			if ui.Button(c, "Back").AlignSelf(ui.Center).Clicked() {
				s.Session = app.LoggedOut{}
			}
		case app.LoggedOut:
			errMsg = sess.Err
			id := field(c, "id", "Email or username", &v.loginID, "").AutoFocus()
			pw := field(c, "pw", "Password", &v.password, "").Password()
			ready := !busy && v.loginID != "" && v.password != ""
			if (primary(c, "Log in", !ready).Clicked() || id.Submitted() || pw.Submitted()) && ready {
				v.App.Login(s, v.loginID, v.password, "")
				v.password = ""
			}
		}
		if errMsg != "" {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				symbol(c, "exclamationmark.circle", 13, t.Danger)
				ui.Text(c, errMsg).FontSize(13).TextColor(t.Danger).Shrink(1)
			})
		}
		if busy {
			ui.Spinner(c).AlignSelf(ui.Center)
		}
	})
}

func serverBadge(s *app.Server) (int64, bool) {
	var mentions int64
	unread := false
	for id := range s.Store.Channels {
		mentions += s.Store.Mentions(id)
		if m := s.Store.Members[id]; m != nil && !m.Muted() && s.Store.Unread(id) {
			unread = true
		}
	}
	return mentions, unread
}

func (v *View) main(c *ui.Context, s *app.Server) {
	v.shortcuts(c, s)
	p := colors(c)
	t := c.Theme()
	ui.Row(c).Grow(1).Background(p.sidebar).Children(func() {
		if len(s.Store.TeamOrder) > 1 {
			v.teamStrip(c, s)
		}
		v.sidebar(c, s)
		// The channel and the RHS sit on a card inset in the sidebar's
		// color, as the webapp draws them.
		ui.Row(c).Grow(1).FillHeight().Margin(0, 4, 4, 0).Radius(8).Clip().Background(t.Background).BorderWidth(1).BorderColor(p.cardBorder).Children(func() {
			if s.Thread != "" {
				ui.SplitPercent(c, &v.rhsSize, func() { v.channel(c, s) }, func() { v.thread(c, s) }).Grow(1).FillHeight().SplitReset(62)
			} else {
				v.channel(c, s)
			}
		})
	})
}

func (v *View) shortcuts(c *ui.Context, s *app.Server) {
	a := v.App
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		v.switcher = switcher{open: true}
	}
	if c.Shortcut(ui.Cmd, ui.KeyBracketLeft) {
		a.Back(s)
	}
	if c.Shortcut(ui.Cmd, ui.KeyBracketRight) {
		a.Forward(s)
	}
	if c.Shortcut(ui.Alt, ui.KeyUp) {
		v.moveChannel(s, -1)
	}
	if c.Shortcut(ui.Alt, ui.KeyDown) {
		v.moveChannel(s, 1)
	}
	if c.Shortcut(0, ui.KeyEscape) && s.Thread != "" {
		a.CloseThread(s)
	}
}

func (v *View) moveChannel(s *app.Server, d int) {
	order := v.sidebarOrder(s)
	for i, id := range order {
		if id == s.Channel {
			if j := i + d; j >= 0 && j < len(order) {
				v.App.OpenChannel(s, order[j])
			}
			return
		}
	}
	if len(order) > 0 {
		v.App.OpenChannel(s, order[0])
	}
}
