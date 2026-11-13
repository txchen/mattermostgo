package view

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// settingsDialog is the user's settings, by tab, as the webapp's
// Settings: notifications, display, sidebar and security.
type settingsDialog struct {
	tab int
	// notify holds the notification settings as the server sent them,
	// edited here until saved.
	notify      map[string]string
	notifyAsked bool
	mentionKeys string
	saved       bool

	current, password, confirm string
	username, email, emailPass string
	mfa                        *mm.MFASecret
	mfaQR                      []byte
	mfaCode                    string
	sessions                   []mm.Session
	tokens                     []mm.AccessToken
	newToken, tokenName        string
	securityAsked              bool
	err                        string
}

func (*settingsDialog) isDialog() {}

var settingsTabs = []string{"Notifications", "Display", "Sidebar", "Security"}

func (v *View) openSettings(s *app.Server, tab int) {
	d := &settingsDialog{tab: tab}
	if me := s.Me(); me != nil {
		d.username, d.email = me.Username, me.Email
	}
	v.dialog = d
}

// choice is a row of a setting: a label, its help, and a control.
func choice(c *ui.Context, label, help string, control func()) {
	t := c.Theme()
	ui.Row(c).Padding(10, 0).Gap(16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, label).FontWeight(600)
			if help != "" {
				ui.Text(c, help).FontSize(12).TextColor(t.TextMuted)
			}
		})
		control()
	})
}

// pick is a Segmented control choosing one value of options for a value
// kept by its key; it calls set with a new one.
func pick(c *ui.Context, key string, value string, options [][2]string, set func(string)) {
	sel := 0
	labels := make([]string, len(options))
	for i, o := range options {
		labels[i] = o[1]
		if o[0] == value {
			sel = i
		}
	}
	prev := sel
	ui.Segmented(c.Key(key), &sel, labels...)
	if sel != prev {
		set(options[sel][0])
	}
}

func (v *View) settingsDialog(c *ui.Context, s *app.Server, d *settingsDialog) {
	ui.Column(c).Width(640).Height(560).Padding(8).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			dialogTitle(c, "Settings")
			ui.Spacer(c)
			if ui.Button(c, "Done").Clicked() {
				v.dialog = nil
			}
		})
		ui.Segmented(c.Key("tabs"), &d.tab, settingsTabs...)
		if d.err != "" {
			ui.Text(c, d.err).FontSize(13).TextColor(c.Theme().Danger)
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 4).Children(func() {
				switch d.tab {
				case 0:
					v.notifySettings(c, s, d)
				case 1:
					v.displaySettings(c, s)
				case 2:
					v.sidebarSettings(c, s)
				case 3:
					v.securitySettings(c, s, d)
				}
			})
		})
	})
}

func (v *View) notifySettings(c *ui.Context, s *app.Server, d *settingsDialog) {
	a := v.App
	if !d.notifyAsked {
		d.notifyAsked = true
		app.Call(a, s, func(ctx context.Context, c *mm.Client) (map[string]string, error) { return c.NotifyProps(ctx) }, func(p map[string]string) {
			d.notify, d.mentionKeys = p, p["mention_keys"]
		})
	}
	if d.notify == nil {
		ui.Spinner(c)
		return
	}
	set := func(k string) func(string) {
		return func(val string) {
			d.notify[k] = val
			a.SetNotify(s, map[string]string{k: val}, nil)
		}
	}
	choice(c, "Desktop notifications", "When macOS shows a notification for a message.", func() {
		pick(c, "desktop", d.notify["desktop"], [][2]string{{"all", "All"}, {"mention", "Mentions"}, {"none", "Never"}}, set("desktop"))
	})
	choice(c, "Thread replies", "Notify for replies in threads you follow.", func() {
		pick(c, "threads", d.notify["desktop_threads"], [][2]string{{"all", "All"}, {"mention", "Mentions"}}, set("desktop_threads"))
	})
	choice(c, "Sound", "Play a sound with each notification.", func() {
		pick(c, "sound", d.notify["desktop_sound"], [][2]string{{"true", "On"}, {"false", "Off"}}, set("desktop_sound"))
	})
	choice(c, "Channel-wide mentions", "Whether @channel, @all and @here notify you.", func() {
		pick(c, "channel", d.notify["channel"], [][2]string{{"true", "On"}, {"false", "Off"}}, set("channel"))
	})
	choice(c, "Your first name", "Count your first name as a mention.", func() {
		pick(c, "first", d.notify["first_name"], [][2]string{{"true", "On"}, {"false", "Off"}}, set("first_name"))
	})
	choice(c, "Other words that mention you", "Separated by commas. Case does not matter.", func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c.Key("keys"), &d.mentionKeys).Label("Mention keywords").Width(200)
			if ui.Button(c, "Save").Disabled(d.mentionKeys == d.notify["mention_keys"]).Clicked() {
				keys := strings.Join(lines(d.mentionKeys), ",")
				d.mentionKeys = keys
				set("mention_keys")(keys)
			}
		})
	})
	choice(c, "Notifications on this Mac", "Allow Mattermost Go in System Settings › Notifications. A test shows whether it can.", func() {
		if ui.Button(c, "Send a test").Clicked() && a.Notify != nil {
			a.Notify(app.Notification{Server: s.Config.ID, Channel: s.Channel, Title: "Mattermost Go", Body: "Notifications work."})
		}
	})
}

// prefSetter saves a preference of the display or the sidebar.
func (v *View) prefSetter(s *app.Server, category, name string) func(string) {
	return func(val string) { v.App.SetPrefs(s, store.Preference{Category: category, Name: name, Value: val}) }
}

func (v *View) displaySettings(c *ui.Context, s *app.Server) {
	t := c.Theme()
	st := s.Store
	disp := func(name, def string) string {
		if p := st.Pref("display_settings", name); p != "" {
			return p
		}
		return def
	}
	ui.Column(c).Padding(10, 0).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, "Theme").FontWeight(600)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			card := func(id, name string, l *look) {
				b := ui.ButtonBase(c.Key("theme-" + id)).Width(104).Padding(6).Radius(6).BorderWidth(2).Label(name + " theme")
				if v.App.Theme == id {
					b.BorderColor(t.Accent)
				} else {
					b.BorderColor(t.Border)
				}
				b.Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						ui.Row(c).FillWidth().Height(44).Radius(4).Clip().Children(func() {
							if l == nil {
								ui.Box(c).Grow(1).Background(lookFor("denim", false).pal.sidebar)
								ui.Box(c).Grow(1).Background(lookFor("onedark", true).pal.sidebar)
								return
							}
							ui.Box(c).Width(28).FillHeight().Background(l.pal.sidebar)
							ui.Column(c).Grow(1).Background(l.bg).Padding(6).Gap(4).Children(func() {
								ui.Box(c).Height(4).Width(40).Radius(2).Background(l.text.Alpha(0.6))
								ui.Box(c).Height(4).Width(28).Radius(2).Background(l.muted.Alpha(0.6))
								ui.Box(c).Height(6).Width(20).Radius(2).Background(l.accent)
							})
						})
						ui.Text(c, name).FontSize(12).TextAlign(ui.Center).FillWidth()
					})
				})
				if b.Clicked() {
					v.App.SetTheme(id)
				}
			}
			card("", "Follow system", nil)
			for i := range looks {
				card(looks[i].id, looks[i].name, &looks[i])
			}
		})
	})
	choice(c, "Clock", "How times show.", func() {
		pick(c, "clock", disp("use_military_time", "true"), [][2]string{{"false", "12-hour"}, {"true", "24-hour"}}, v.prefSetter(s, "display_settings", "use_military_time"))
	})
	choice(c, "Teammate names", "How people show in posts and the sidebar.", func() {
		pick(c, "names", disp("name_format", "username"), [][2]string{{"username", "Username"}, {"nickname_full_name", "Nickname"}, {"full_name", "Full name"}}, v.prefSetter(s, "display_settings", "name_format"))
	})
	choice(c, "Message display", "Compact leaves out pictures and packs posts closer.", func() {
		pick(c, "display", disp("message_display", "clean"), [][2]string{{"clean", "Standard"}, {"compact", "Compact"}}, v.prefSetter(s, "display_settings", "message_display"))
	})
	choice(c, "Link previews", "Show a preview of the first link in a message.", func() {
		pick(c, "previews", disp("link_previews", "true"), [][2]string{{"true", "On"}, {"false", "Off"}}, v.prefSetter(s, "display_settings", "link_previews"))
	})
	choice(c, "Click to open threads", "A click on a message opens its thread.", func() {
		pick(c, "click", disp("click_to_reply", "true"), [][2]string{{"true", "On"}, {"false", "Off"}}, v.prefSetter(s, "display_settings", "click_to_reply"))
	})
}

func (v *View) sidebarSettings(c *ui.Context, s *app.Server) {
	st := s.Store
	unread := st.Pref("sidebar_settings", "show_unread_section")
	if unread == "" {
		unread = "false"
	}
	choice(c, "Group unread channels separately", "Unread channels show together at the top of the sidebar.", func() {
		pick(c, "unread", unread, [][2]string{{"true", "On"}, {"false", "Off"}}, v.prefSetter(s, "sidebar_settings", "show_unread_section"))
	})
	limit := st.Pref("sidebar_settings", "limit_visible_dms_gms")
	if limit == "" {
		limit = "40"
	}
	choice(c, "Direct messages to show", "The most recent ones; the others stay a click away.", func() {
		pick(c, "dms", limit, [][2]string{{"10", "10"}, {"15", "15"}, {"20", "20"}, {"40", "40"}, {"10000", "All"}}, v.prefSetter(s, "sidebar_settings", "limit_visible_dms_gms"))
	})
}

func (v *View) securitySettings(c *ui.Context, s *app.Server, d *settingsDialog) {
	t := c.Theme()
	a := v.App
	me := s.Me()
	if me == nil {
		return
	}
	if !d.securityAsked {
		d.securityAsked = true
		v.loadSessions(s, d)
		app.Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.AccessToken, error) { return c.Tokens(ctx, string(me.ID)) },
			func(ts []mm.AccessToken) { d.tokens = ts })
	}
	heading := func(text string) {
		ui.Text(c, text).FontSize(11).FontWeight(600).TextColor(t.TextMuted).Margin(14, 0, 4, 0)
	}
	heading("USERNAME AND EMAIL")
	field(c, "username", "Username", &d.username, "")
	field(c, "email", "Email", &d.email, "")
	if d.email != me.Email {
		field(c, "emailpass", "Password, to change the email", &d.emailPass, "").Password()
	}
	ui.Row(c).Justify(ui.End).Margin(6, 0, 0, 0).Children(func() {
		changed := d.username != me.Username || d.email != me.Email
		if ui.PrimaryButton(c, "Save").Disabled(!changed).Clicked() && changed {
			username, email, pass := strings.TrimSpace(d.username), "", d.emailPass
			if d.email != me.Email {
				email = strings.TrimSpace(d.email)
			}
			d.err = ""
			app.Call(a, s, func(ctx context.Context, c *mm.Client) (store.User, error) {
				return c.PatchAccount(ctx, username, email, pass)
			}, func(u store.User) {
				a.RefreshMe(s)
				d.emailPass = ""
				v.ShowError("Saved")
			})
		}
	})

	heading("PASSWORD")
	field(c, "current", "Current password", &d.current, "").Password()
	field(c, "new", "New password", &d.password, "").Password()
	field(c, "confirm", "New password again", &d.confirm, "").Password()
	ui.Row(c).Justify(ui.End).Margin(6, 0, 0, 0).Children(func() {
		ok := d.current != "" && d.password != "" && d.password == d.confirm
		if d.password != "" && d.confirm != "" && d.password != d.confirm {
			ui.Text(c, "The new passwords differ.").FontSize(12).TextColor(t.Danger).Grow(1)
		}
		if ui.PrimaryButton(c, "Change Password").Disabled(!ok).Clicked() && ok {
			cur, next, id := d.current, d.password, me.ID
			app.Call(a, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
				return struct{}{}, c.ChangePassword(ctx, id, cur, next)
			}, func(struct{}) {
				d.current, d.password, d.confirm = "", "", ""
				v.ShowError("Password changed")
			})
		}
	})

	heading("MULTI-FACTOR AUTHENTICATION")
	switch {
	case d.mfa != nil:
		ui.Text(c, "Scan the code with your authenticator app, or enter the secret, then type the code it shows.").FontSize(13)
		qr := d.mfaQR
		if bmp := v.Media.Get("mfa:"+d.mfa.Secret, 360, func(context.Context) ([]byte, error) { return qr, nil }); bmp != nil {
			ui.Image(c, bmp).Size(180, 180)
		}
		ui.Text(c, d.mfa.Secret).Font("monospace").Selectable()
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c.Key("mfacode"), &d.mfaCode).Label("Code").Placeholder("123456").Width(120)
			if ui.PrimaryButton(c, "Turn On").Disabled(len(d.mfaCode) < 6).Clicked() {
				code, id := d.mfaCode, me.ID
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
					return struct{}{}, c.SetMFA(ctx, id, true, code)
				}, func(struct{}) {
					d.mfa, d.mfaCode = nil, ""
					a.RefreshMe(s)
					v.ShowError("Multi-factor authentication is on")
				})
			}
		})
	case me.MFAActive:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "On. Logging in asks for a code from your authenticator.").Grow(1)
			if ui.Button(c, "Turn Off").Clicked() {
				id := me.ID
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
					return struct{}{}, c.SetMFA(ctx, id, false, "")
				}, func(struct{}) { a.RefreshMe(s) })
			}
		})
	default:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Off.").Grow(1)
			if ui.Button(c, "Set Up").Clicked() {
				id := me.ID
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.MFASecret, error) { return c.GenerateMFA(ctx, id) }, func(m mm.MFASecret) {
					d.mfa, d.mfaQR = &m, m.QRCode
				})
			}
		})
	}

	heading("ACTIVE SESSIONS")
	// The session active last is this one, which asked for the list.
	var newest int64
	for _, ss := range d.sessions {
		newest = max(newest, ss.LastActivityAt)
	}
	for _, ss := range d.sessions {
		ui.Row(c.Key(ss.ID)).Gap(8).Padding(4, 0).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				what := strings.TrimSpace(ss.Props.Platform + " " + ss.Props.OS + " " + ss.Props.Browser)
				if what == "" {
					what = "Unknown device"
				}
				ui.Text(c, what).FontWeight(600).MaxLines(1)
				ui.Text(c, "Last active "+ago(c.Now(), ss.LastActivityAt)+" · since "+time.UnixMilli(ss.CreateAt).Format("Jan 2, 2006")).FontSize(12).TextColor(t.TextMuted)
			})
			if ss.LastActivityAt == newest {
				ui.Text(c, "This device").FontSize(12).TextColor(t.TextMuted)
			} else if ui.Button(c, "Log Out").Label("Log out session " + ss.ID).Clicked() {
				id, sid := me.ID, ss.ID
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
					return struct{}{}, c.RevokeSession(ctx, id, sid)
				}, func(struct{}) { v.loadSessions(s, d) })
			}
		})
	}

	heading("PERSONAL ACCESS TOKENS")
	if d.newToken != "" {
		ui.Row(c).Gap(8).Padding(8).Radius(4).Background(t.Surface).AlignItems(ui.Center).Children(func() {
			ui.Text(c, d.newToken).Font("monospace").Grow(1).Selectable()
			if ui.Button(c, "Copy").Clicked() {
				c.WriteClipboard(d.newToken)
			}
		})
		ui.Text(c, "Copy the token now: it will not show again.").FontSize(12).TextColor(t.TextMuted)
	}
	for _, tok := range d.tokens {
		if !tok.IsActive {
			continue
		}
		ui.Row(c.Key(tok.ID)).Gap(8).Padding(4, 0).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tok.Description).Grow(1)
			if ui.Button(c, "Delete").Label("Delete token " + tok.Description).Clicked() {
				tid, uid := tok.ID, string(me.ID)
				app.Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.AccessToken, error) {
					if err := c.RevokeToken(ctx, tid); err != nil {
						return nil, err
					}
					return c.Tokens(ctx, uid)
				}, func(ts []mm.AccessToken) { d.tokens = ts })
			}
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c.Key("tokname"), &d.tokenName).Label("Token description").Placeholder("What the token is for").Grow(1)
		if ui.Button(c, "Create Token").Disabled(strings.TrimSpace(d.tokenName) == "").Clicked() {
			desc, uid := strings.TrimSpace(d.tokenName), string(me.ID)
			app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.AccessToken, error) { return c.CreateToken(ctx, uid, desc) }, func(tok mm.AccessToken) {
				d.newToken, d.tokenName = tok.Token, ""
				d.tokens = append(d.tokens, tok)
			})
		}
	})
}

func (v *View) loadSessions(s *app.Server, d *settingsDialog) {
	id := s.Store.Me
	app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]mm.Session, error) { return c.Sessions(ctx, id) }, func(ss []mm.Session) { d.sessions = ss })
}

// changePicture asks for an image and makes it the profile picture.
func (v *View) changePicture(s *app.Server) {
	if v.Hooks.PickFiles == nil {
		return
	}
	v.Hooks.PickFiles(func(paths []string) {
		if len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			v.ShowError(err.Error())
			return
		}
		id, name := s.Store.Me, filepath.Base(paths[0])
		app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
			return struct{}{}, c.SetProfileImage(ctx, id, name, data)
		},
			func(struct{}) { v.App.RefreshMe(s) })
	})
}

// channelNotifyDialog is the notification settings of one channel.
type channelNotifyDialog struct {
	channel store.ChannelID
	props   map[string]string
}

func (*channelNotifyDialog) isDialog() {}

func (v *View) openChannelNotify(s *app.Server, id store.ChannelID) {
	d := &channelNotifyDialog{channel: id}
	v.dialog = d
	app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (map[string]string, error) {
		return c.ChannelNotifyProps(ctx, id)
	},
		func(p map[string]string) { d.props = p })
}

func (v *View) channelNotifyDialog(c *ui.Context, s *app.Server, d *channelNotifyDialog) {
	a := v.App
	ch := s.Store.Channels[d.channel]
	if ch == nil {
		v.dialog = nil
		return
	}
	ui.Column(c).Width(560).Padding(8).Gap(8).Children(func() {
		dialogTitle(c, "Notification Preferences · "+a.ChannelName(s, ch))
		if d.props == nil {
			ui.Spinner(c)
			return
		}
		set := func(k string) func(string) {
			return func(val string) {
				d.props[k] = val
				a.SetChannelNotify(s, d.channel, map[string]string{k: val}, nil)
			}
		}
		choice(c, "Mute channel", "Muted channels do not show as unread and do not notify, except for mentions.", func() {
			pick(c, "mute", d.props["mark_unread"], [][2]string{{"all", "Off"}, {"mention", "On"}}, set("mark_unread"))
		})
		choice(c, "Desktop notifications", "Default follows your settings.", func() {
			pick(c, "desktop", d.props["desktop"], [][2]string{{"default", "Default"}, {"all", "All"}, {"mention", "Mentions"}, {"none", "Never"}}, set("desktop"))
		})
		choice(c, "Ignore @channel, @here and @all", "They then do not count as mentions here.", func() {
			pick(c, "ignore", d.props["ignore_channel_mentions"], [][2]string{{"default", "Default"}, {"off", "Off"}, {"on", "On"}}, set("ignore_channel_mentions"))
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			if ui.Button(c, "Done").Clicked() {
				v.dialog = nil
			}
		})
	})
}
