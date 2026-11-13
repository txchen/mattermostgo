package view

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// integrationTab is a tab of the integrations dialog.
type integrationTab int

const (
	tabIncoming integrationTab = iota
	tabOutgoing
	tabCommands
	tabBots
)

var integrationTabs = []string{"Incoming Webhooks", "Outgoing Webhooks", "Slash Commands", "Bot Accounts"}

// integrations is the dialog managing a team's webhooks, slash commands
// and the server's bot accounts, as the webapp's Integrations pages.
type integrations struct {
	team     store.TeamID
	tab      int
	asked    map[integrationTab]bool
	loaded   map[integrationTab]bool
	incoming []mm.IncomingHook
	outgoing []mm.OutgoingHook
	commands []mm.Command
	bots     []mm.Bot
	// adding is the form of the tab's new integration, nil while the
	// list shows.
	adding form
	// secret is what the server shows once, as a new hook's URL or a
	// bot's token, with what it belongs to.
	secret, secretFor string
	err               string
	// submit is the button of the open form, as it built last.
	submit formSubmit
}

type formSubmit struct {
	label string
	ok    bool
	work  func()
}

type form interface{ isForm() }

type incomingForm struct {
	hook    mm.IncomingHook
	channel string
}

type outgoingForm struct {
	hook             mm.OutgoingHook
	channel          string
	words, urls      string
	json, startsWith bool
}

type commandForm struct {
	cmd  mm.Command
	post bool
}

type botForm struct{ bot mm.Bot }

func (*incomingForm) isForm() {}
func (*outgoingForm) isForm() {}
func (*commandForm) isForm()  {}
func (*botForm) isForm()      {}

func (v *View) openIntegrations(s *app.Server) {
	v.dialog = &integrations{team: s.Team, asked: map[integrationTab]bool{}, loaded: map[integrationTab]bool{}}
}

func (v *View) loadIntegrations(s *app.Server, d *integrations, tab integrationTab) {
	team := d.team
	switch tab {
	case tabIncoming:
		app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]mm.IncomingHook, error) { return c.IncomingHooks(ctx, team) },
			func(hs []mm.IncomingHook) {
				d.incoming, d.loaded[tab] = newestFirst(hs, func(h mm.IncomingHook) int64 { return h.CreateAt }), true
			})
	case tabOutgoing:
		app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]mm.OutgoingHook, error) { return c.OutgoingHooks(ctx, team) },
			func(hs []mm.OutgoingHook) {
				d.outgoing, d.loaded[tab] = newestFirst(hs, func(h mm.OutgoingHook) int64 { return h.CreateAt }), true
			})
	case tabCommands:
		app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]mm.Command, error) { return c.Commands(ctx, team) },
			func(cs []mm.Command) {
				d.commands, d.loaded[tab] = newestFirst(cs, func(c mm.Command) int64 { return c.CreateAt }), true
			})
	case tabBots:
		app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]mm.Bot, error) { return c.Bots(ctx) },
			func(bs []mm.Bot) {
				d.bots, d.loaded[tab] = newestFirst(bs, func(b mm.Bot) int64 { return b.CreateAt }), true
			})
	}
}

// newestFirst sorts integrations by when they were made, newest first.
func newestFirst[T any](xs []T, at func(T) int64) []T {
	slices.SortStableFunc(xs, func(a, b T) int { return cmp.Compare(at(b), at(a)) })
	return xs
}

// teamChannels are the channels of the team a hook may post in, by name.
func (v *View) teamChannels(s *app.Server, team store.TeamID) []*store.Channel {
	var out []*store.Channel
	for _, ch := range s.Store.Channels {
		if ch.TeamID == team && (ch.Type == store.Open || ch.Type == store.Private) {
			out = append(out, ch)
		}
	}
	slices.SortFunc(out, func(a, b *store.Channel) int {
		return cmp.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})
	return out
}

func (v *View) channelByDisplay(s *app.Server, team store.TeamID, name string) string {
	for _, ch := range v.teamChannels(s, team) {
		if ch.DisplayName == name {
			return string(ch.ID)
		}
	}
	return ""
}

func (v *View) channelLabel(s *app.Server, id string) string {
	if ch := s.Store.Channels[store.ChannelID(id)]; ch != nil {
		return ch.DisplayName
	}
	if id == "" {
		return "Any channel"
	}
	return "A channel you are not in"
}

func (v *View) integrationsDialog(c *ui.Context, s *app.Server, d *integrations) {
	t := c.Theme()
	ui.Column(c).Width(720).Height(560).Padding(8).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			title := "Integrations"
			if team := s.Store.Teams[d.team]; team != nil {
				title += " · " + team.DisplayName
			}
			dialogTitle(c, title)
			ui.Spacer(c)
			if ui.Button(c, "Done").Clicked() {
				v.dialog = nil
			}
		})
		prev := d.tab
		ui.Segmented(c.Key("tabs"), &d.tab, integrationTabs...)
		tab := integrationTab(d.tab)
		if d.tab != prev {
			d.adding, d.secret, d.err = nil, "", ""
		}
		if !d.asked[tab] {
			d.asked[tab] = true
			v.loadIntegrations(s, d, tab)
		}
		if d.secret != "" {
			ui.Column(c).Padding(12).Gap(6).Radius(6).Background(t.Surface).Children(func() {
				ui.Text(c, d.secretFor).FontWeight(600)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, d.secret).Font("monospace").FontSize(12).Selectable().Grow(1).Shrink(1)
					if ui.Button(c, "Copy").Clicked() {
						c.WriteClipboard(d.secret)
						c.Toast("Copied")
					}
				})
			})
		}
		if d.err != "" {
			ui.Text(c, d.err).FontSize(13).TextColor(t.Danger)
		}
		ui.Column(c).Grow(1).Children(func() {
			if d.adding != nil {
				ui.Scroll(c).Grow(1).Children(func() { v.integrationForm(c, s, d) })
				f := d.submit
				v.formButtons(c, d, f.label, f.ok, f.work)
				return
			}
			ui.Row(c).Children(func() {
				ui.Spacer(c)
				if ui.PrimaryButton(c, "Add "+strings.TrimSuffix(integrationTabs[d.tab], "s")).Clicked() {
					d.secret, d.err = "", ""
					d.adding = newForm(tab)
				}
			})
			ui.Scroll(c).Grow(1).Margin(8, 0, 0, 0).Children(func() { v.integrationList(c, s, d, tab) })
		})
	})
}

func newForm(tab integrationTab) form {
	switch tab {
	case tabIncoming:
		return &incomingForm{hook: mm.IncomingHook{ChannelLocked: true}}
	case tabOutgoing:
		return &outgoingForm{json: true}
	case tabCommands:
		return &commandForm{post: true}
	}
	return &botForm{}
}

// integrationRow is a row of a list: a title, details and actions.
func integrationRow(c *ui.Context, key any, title string, details []string, actions func()) {
	t := c.Theme()
	ui.Row(c.Key(key)).Padding(10, 4).Gap(12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, title).FontWeight(600).MaxLines(1)
			for _, d := range details {
				if d != "" {
					ui.Text(c, d).FontSize(12).TextColor(t.TextMuted).MaxLines(1).Selectable()
				}
			}
		})
		ui.Row(c).Gap(6).Children(actions)
	})
}

func emptyList(c *ui.Context, loaded bool, what string) {
	ui.Column(c).Padding(40).Center().Children(func() {
		if !loaded {
			ui.Spinner(c)
			return
		}
		ui.Text(c, "No "+what+" yet").TextColor(c.Theme().TextMuted)
	})
}

func (v *View) integrationList(c *ui.Context, s *app.Server, d *integrations, tab integrationTab) {
	a := v.App
	cl := s.Client()
	if cl == nil {
		return
	}
	loaded := d.loaded[tab]
	switch tab {
	case tabIncoming:
		if len(d.incoming) == 0 {
			emptyList(c, loaded, "incoming webhooks")
		}
		for _, h := range d.incoming {
			integrationRow(c, h.ID, cmp.Or(h.DisplayName, "Unnamed webhook"), []string{h.Description, "Posts to " + v.channelLabel(s, h.ChannelID), cl.HookURL(h.ID)}, func() {
				if ui.Button(c, "Edit").Label("Edit " + h.DisplayName).Clicked() {
					d.secret, d.err = "", ""
					d.adding = &incomingForm{hook: h, channel: v.channelLabel(s, h.ChannelID)}
				}
				if ui.Button(c, "Copy URL").Label("Copy URL of " + h.DisplayName).Clicked() {
					c.WriteClipboard(cl.HookURL(h.ID))
					c.Toast("URL copied")
				}
				if ui.Button(c, "Delete").Label("Delete " + h.DisplayName).Clicked() {
					v.confirmDelete(s, d, "the webhook "+h.DisplayName, func(ctx context.Context, c *mm.Client) error { return c.DeleteIncomingHook(ctx, h.ID) }, tab)
				}
			})
		}
	case tabOutgoing:
		if len(d.outgoing) == 0 {
			emptyList(c, loaded, "outgoing webhooks")
		}
		for _, h := range d.outgoing {
			integrationRow(c, h.ID, cmp.Or(h.DisplayName, "Unnamed webhook"), []string{h.Description, "Trigger words: " + strings.Join(h.TriggerWords, ", ") + " · in " + v.channelLabel(s, h.ChannelID), strings.Join(h.CallbackURLs, ", ")}, func() {
				if ui.Button(c, "Edit").Label("Edit " + h.DisplayName).Clicked() {
					d.secret, d.err = "", ""
					d.adding = &outgoingForm{hook: h, channel: v.channelLabel(s, h.ChannelID), words: strings.Join(h.TriggerWords, ", "),
						urls: strings.Join(h.CallbackURLs, "\n"), json: h.ContentType == "application/json", startsWith: h.TriggerWhen == 1}
				}
				if ui.Button(c, "Copy Token").Label("Copy token of " + h.DisplayName).Clicked() {
					c.WriteClipboard(h.Token)
					c.Toast("Token copied")
				}
				if ui.Button(c, "Delete").Label("Delete " + h.DisplayName).Clicked() {
					v.confirmDelete(s, d, "the webhook "+h.DisplayName, func(ctx context.Context, c *mm.Client) error { return c.DeleteOutgoingHook(ctx, h.ID) }, tab)
				}
			})
		}
	case tabCommands:
		if len(d.commands) == 0 {
			emptyList(c, loaded, "slash commands")
		}
		for _, cmd := range d.commands {
			integrationRow(c, cmd.ID, cmp.Or(cmd.DisplayName, "/"+cmd.Trigger), []string{"/" + cmd.Trigger + " · " + cmd.Description, cmd.URL}, func() {
				if ui.Button(c, "Edit").Label("Edit /" + cmd.Trigger).Clicked() {
					d.secret, d.err = "", ""
					d.adding = &commandForm{cmd: cmd, post: cmd.Method != "G"}
				}
				if ui.Button(c, "Copy Token").Label("Copy token of /" + cmd.Trigger).Clicked() {
					c.WriteClipboard(cmd.Token)
					c.Toast("Token copied")
				}
				if ui.Button(c, "Delete").Label("Delete /" + cmd.Trigger).Clicked() {
					v.confirmDelete(s, d, "the command /"+cmd.Trigger, func(ctx context.Context, c *mm.Client) error { return c.DeleteCommand(ctx, cmd.ID) }, tab)
				}
			})
		}
	case tabBots:
		if len(d.bots) == 0 {
			emptyList(c, loaded, "bot accounts")
		}
		for _, b := range d.bots {
			state := ""
			if b.DeleteAt != 0 {
				state = "Disabled"
			}
			owner := ""
			if b.OwnerID != "" {
				owner = "Managed by " + a.UserName(s, store.UserID(b.OwnerID))
			}
			integrationRow(c, b.UserID, cmp.Or(b.DisplayName, b.Username)+" (@"+b.Username+")", []string{b.Description, owner, state}, func() {
				if ui.Button(c, "Edit").Label("Edit " + b.Username).Clicked() {
					d.secret, d.err = "", ""
					d.adding = &botForm{bot: b}
				}
				if b.DeleteAt == 0 && ui.Button(c, "Create Token").Label("Create token for "+b.Username).Clicked() {
					v.createBotToken(s, d, b)
				}
				label := "Disable"
				if b.DeleteAt != 0 {
					label = "Enable"
				}
				if ui.Button(c, label).Label(label + " " + b.Username).Clicked() {
					enable := b.DeleteAt != 0
					app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.Bot, error) { return c.SetBotEnabled(ctx, b.UserID, enable) }, func(nb mm.Bot) {
						for i := range d.bots {
							if d.bots[i].UserID == nb.UserID {
								d.bots[i] = nb
							}
						}
					})
				}
			})
		}
	}
}

func (v *View) confirmDelete(s *app.Server, d *integrations, what string, work func(ctx context.Context, c *mm.Client) error, tab integrationTab) {
	v.dialog = &confirmDialog{title: "Delete " + what + "?", action: "Delete", body: "Whatever uses it stops working.", do: func() {
		v.dialog = d
		app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (struct{}, error) { return struct{}{}, work(ctx, c) }, func(struct{}) {
			v.loadIntegrations(s, d, tab)
		})
	}}
}

func (v *View) createBotToken(s *app.Server, d *integrations, b mm.Bot) {
	app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (mm.AccessToken, error) {
		return c.CreateToken(ctx, b.UserID, "Made in Mattermost Go")
	},
		func(tok mm.AccessToken) {
			d.secret, d.secretFor = tok.Token, "Access token for @"+b.Username+". Copy it now: it will not show again."
		})
}

// lines splits what was typed into its items, one a line or between commas.
func lines(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' }) {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (v *View) channelSelect(c *ui.Context, s *app.Server, d *integrations, value *string, any bool) {
	names := []string{}
	if any {
		names = append(names, "Any channel")
	}
	for _, ch := range v.teamChannels(s, d.team) {
		names = append(names, ch.DisplayName)
	}
	if *value == "" && len(names) > 0 {
		*value = names[0]
	}
	ui.Text(c, "Channel").FontSize(13).FontWeight(600).Margin(4, 0, 0, 0)
	ui.Select(c.Key("channel"), value, names).Label("Channel")
}

func (v *View) integrationForm(c *ui.Context, s *app.Server, d *integrations) {
	a := v.App
	team := d.team
	// The buttons go under the scrolling form, so that they always show.
	submit := func(label string, ok bool, work func()) {
		d.submit = formSubmit{label, ok, work}
	}
	ui.Column(c).Gap(6).Padding(0, 4).Children(func() {
		switch f := d.adding.(type) {
		case *incomingForm:
			field(c, "name", "Title", &f.hook.DisplayName, "Build alerts").AutoFocus()
			field(c, "desc", "Description", &f.hook.Description, "Optional")
			v.channelSelect(c, s, d, &f.channel, false)
			ui.Checkbox(c.Key("lock"), &f.hook.ChannelLocked, "Lock to this channel").Margin(4, 0)
			field(c, "user", "Username", &f.hook.Username, "Optional, as the posts show it")
			field(c, "icon", "Profile picture URL", &f.hook.IconURL, "Optional")
			submit("Save", f.hook.DisplayName != "" && f.channel != "", func() {
				h := f.hook
				if id := v.channelByDisplay(s, team, f.channel); id != "" {
					h.ChannelID = id
				}
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.IncomingHook, error) {
					if h.ID != "" {
						return c.UpdateIncomingHook(ctx, h)
					}
					return c.CreateIncomingHook(ctx, h)
				}, func(nh mm.IncomingHook) {
					d.secret, d.secretFor = s.Client().HookURL(nh.ID), "Webhook URL for "+nh.DisplayName+". Post JSON such as {\"text\": \"Hello\"} to it."
					d.adding = nil
					v.loadIntegrations(s, d, tabIncoming)
				})
			})
		case *outgoingForm:
			field(c, "name", "Title", &f.hook.DisplayName, "Deploy trigger").AutoFocus()
			field(c, "desc", "Description", &f.hook.Description, "Optional")
			ui.Checkbox(c.Key("json"), &f.json, "Send JSON (else form data)").Margin(4, 0)
			v.channelSelect(c, s, d, &f.channel, true)
			field(c, "words", "Trigger words", &f.words, "One a line or comma separated")
			ui.Checkbox(c.Key("starts"), &f.startsWith, "Trigger when the first word starts with a trigger word").Margin(4, 0)
			ui.Text(c, "Callback URLs").FontSize(13).FontWeight(600).Margin(4, 0, 0, 0).AccessHidden(true)
			ui.TextArea(c.Key("urls"), &f.urls).Lines(2, 5).Label("Callback URLs").Placeholder("One a line")
			submit("Save", f.hook.DisplayName != "" && len(lines(f.urls)) > 0 && (len(lines(f.words)) > 0 || f.channel != "Any channel"), func() {
				h := f.hook
				h.TeamID = string(team)
				switch id := v.channelByDisplay(s, team, f.channel); {
				case f.channel == "Any channel":
					h.ChannelID = ""
				case id != "":
					h.ChannelID = id
				}
				h.TriggerWords, h.CallbackURLs = lines(f.words), lines(f.urls)
				h.ContentType = "application/x-www-form-urlencoded"
				if f.json {
					h.ContentType = "application/json"
				}
				if f.startsWith {
					h.TriggerWhen = 1
				}
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.OutgoingHook, error) {
					if h.ID != "" {
						return c.UpdateOutgoingHook(ctx, h)
					}
					return c.CreateOutgoingHook(ctx, h)
				}, func(nh mm.OutgoingHook) {
					d.secret, d.secretFor = nh.Token, "Token of "+nh.DisplayName+": requests to the callback URLs carry it."
					d.adding = nil
					v.loadIntegrations(s, d, tabOutgoing)
				})
			})
		case *commandForm:
			field(c, "name", "Title", &f.cmd.DisplayName, "Weather").AutoFocus()
			field(c, "desc", "Description", &f.cmd.Description, "Optional")
			field(c, "trigger", "Trigger word", &f.cmd.Trigger, "weather, without the slash")
			field(c, "url", "Request URL", &f.cmd.URL, "https://example.com/command")
			ui.Checkbox(c.Key("post"), &f.post, "Send a POST request (else GET)").Margin(4, 0)
			field(c, "user", "Response username", &f.cmd.Username, "Optional")
			field(c, "icon", "Response icon URL", &f.cmd.IconURL, "Optional")
			ui.Checkbox(c.Key("auto"), &f.cmd.AutoComplete, "Show in autocomplete").Margin(4, 0)
			if f.cmd.AutoComplete {
				field(c, "hint", "Autocomplete hint", &f.cmd.AutoCompleteHint, "[city]")
				field(c, "adesc", "Autocomplete description", &f.cmd.AutoCompleteDesc, "Optional")
			}
			trigger := strings.TrimPrefix(strings.TrimSpace(f.cmd.Trigger), "/")
			submit("Save", f.cmd.DisplayName != "" && trigger != "" && f.cmd.URL != "", func() {
				cmd := f.cmd
				cmd.TeamID, cmd.Trigger, cmd.Method = string(team), trigger, "G"
				if f.post {
					cmd.Method = "P"
				}
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.Command, error) {
					if cmd.ID != "" {
						return c.UpdateCommand(ctx, cmd)
					}
					return c.CreateCommand(ctx, cmd)
				}, func(nc mm.Command) {
					d.secret, d.secretFor = nc.Token, "Token of /"+nc.Trigger+": requests to its URL carry it."
					d.adding = nil
					v.loadIntegrations(s, d, tabCommands)
				})
			})
		case *botForm:
			field(c, "user", "Username", &f.bot.Username, "lowercase, as deploy-bot").AutoFocus()
			field(c, "name", "Display name", &f.bot.DisplayName, "Optional")
			field(c, "desc", "Description", &f.bot.Description, "Optional")
			if f.bot.UserID != "" {
				submit("Save", f.bot.Username != "", func() {
					b := f.bot
					app.Call(a, s, func(ctx context.Context, c *mm.Client) (mm.Bot, error) { return c.UpdateBot(ctx, b) }, func(mm.Bot) {
						d.adding = nil
						v.loadIntegrations(s, d, tabBots)
					})
				})
				return
			}
			ui.Text(c, "The bot joins this team, and gets an access token, shown once.").FontSize(12).TextColor(c.Theme().TextMuted)
			submit("Create Bot Account", f.bot.Username != "", func() {
				b := f.bot
				b.Username = strings.ToLower(strings.TrimSpace(b.Username))
				type made struct {
					bot mm.Bot
					tok mm.AccessToken
				}
				app.Call(a, s, func(ctx context.Context, c *mm.Client) (made, error) {
					nb, err := c.CreateBot(ctx, b)
					if err != nil {
						return made{}, err
					}
					if err := c.AddToTeam(ctx, team, nb.UserID); err != nil {
						return made{bot: nb}, err
					}
					tok, err := c.CreateToken(ctx, nb.UserID, "Made in Mattermost Go")
					return made{nb, tok}, err
				}, func(m made) {
					d.secret, d.secretFor = m.tok.Token, "Access token for @"+m.bot.Username+". Copy it now: it will not show again."
					d.adding = nil
					v.loadIntegrations(s, d, tabBots)
				})
			})
		}
	})
}

func (v *View) formButtons(c *ui.Context, d *integrations, label string, ok bool, save func()) {
	ui.Row(c).Gap(8).Justify(ui.End).Margin(12, 0, 0, 0).Children(func() {
		if ui.Button(c, "Cancel").Clicked() {
			d.adding, d.err = nil, ""
		}
		if ui.PrimaryButton(c, label).Disabled(!ok).Clicked() && ok {
			d.err = ""
			save()
		}
	})
}
