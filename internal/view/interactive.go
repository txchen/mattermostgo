package view

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// interactiveDialog is a dialog an integration opened, with what the
// user typed and the errors the integration answered with.
type interactiveDialog struct {
	d      store.Dialog
	text   map[string]*string
	on     map[string]*bool
	errors map[string]string
	busy   bool
}

func (d *interactiveDialog) show(v *View, c *ui.Context, s *app.Server) { v.interactiveDialog(c, s, d) }

func newInteractive(d store.Dialog) *interactiveDialog {
	id := &interactiveDialog{d: d, text: map[string]*string{}, on: map[string]*bool{}, errors: map[string]string{}}
	for _, e := range d.Elements {
		def := e.Default
		id.text[e.Name] = &def
		on := e.Default == "true"
		id.on[e.Name] = &on
	}
	return id
}

// takeDialog shows the dialog an integration asked for, unless another
// dialog shows.
func (v *View) takeDialog(s *app.Server) {
	if v.dialog != nil || s == nil || !s.Ready {
		return
	}
	if d := v.App.TakeDialog(s); d != nil {
		v.dialog = newInteractive(*d)
	}
}

// values are what the dialog sends: text, or a boolean for a checkbox.
func (d *interactiveDialog) values() (map[string]any, map[string]string) {
	out, missing := map[string]any{}, map[string]string{}
	for _, e := range d.d.Elements {
		if e.Type == "bool" {
			out[e.Name] = *d.on[e.Name]
			continue
		}
		val := strings.TrimSpace(*d.text[e.Name])
		switch {
		case val == "" && !e.Optional:
			missing[e.Name] = "This field is required."
		case e.MinLength > 0 && len([]rune(val)) < e.MinLength && val != "":
			missing[e.Name] = "Too short."
		}
		if val != "" {
			out[e.Name] = val
		}
	}
	return out, missing
}

func (v *View) interactiveDialog(c *ui.Context, s *app.Server, d *interactiveDialog) {
	t := c.Theme()
	ui.Column(c).Width(520).Padding(8).Gap(8).Children(func() {
		dialogTitle(c, d.d.Title)
		if d.d.Introduction != "" {
			ui.Text(c, d.d.Introduction).TextColor(t.TextMuted)
		}
		if msg := d.errors[""]; msg != "" {
			ui.Text(c, msg).FontSize(13).TextColor(t.Danger)
		}
		ui.Column(c).MaxHeight(440).Children(func() {
			ui.Scroll(c).Children(func() {
				ui.Column(c).Gap(4).Padding(0, 2).Children(func() {
					for i, e := range d.d.Elements {
						ui.Column(c.Key(i)).Gap(4).Children(func() { v.dialogElement(c, s, d, e) })
					}
				})
			})
		})
		label := d.d.SubmitLabel
		if label == "" {
			label = "Submit"
		}
		ui.Row(c).Gap(8).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
			if ui.Button(c, "Cancel").Clicked() {
				v.dialog = nil
				if d.d.NotifyOnCancel {
					v.App.SubmitDialog(s, d.d, nil, true, func(map[string]string) {})
				}
			}
			if ui.PrimaryButton(c, label).Disabled(d.busy).Clicked() && !d.busy {
				values, missing := d.values()
				d.errors = missing
				if len(missing) > 0 {
					return
				}
				d.busy = true
				v.App.SubmitDialog(s, d.d, values, false, func(errs map[string]string) {
					d.busy = false
					if len(errs) > 0 {
						d.errors = errs
						return
					}
					if v.dialog == d {
						v.dialog = nil
					}
				})
			}
		})
	})
}

func (v *View) dialogElement(c *ui.Context, s *app.Server, d *interactiveDialog, e store.DialogElement) {
	t := c.Theme()
	name := e.DisplayName
	if !e.Optional && e.Type != "bool" {
		name += " *"
	}
	ui.Text(c, name).FontSize(13).FontWeight(600).Margin(6, 0, 0, 0)
	switch e.Type {
	case "textarea":
		ui.TextArea(c.Key("v"), d.text[e.Name]).Lines(3, 8).Label(e.DisplayName).Placeholder(e.Placeholder)
	case "select", "radio":
		opts := v.actionOptions(s, e.DataSource, e.Options)
		labels := make([]string, 0, len(opts)+1)
		if e.Optional || *d.text[e.Name] == "" {
			labels = append(labels, "")
		}
		sel := ""
		for _, o := range opts {
			labels = append(labels, o.Text)
			if o.Value == *d.text[e.Name] {
				sel = o.Text
			}
		}
		prev := sel
		ui.Select(c.Key("v"), &sel, labels).Label(e.DisplayName)
		if sel != prev {
			*d.text[e.Name] = ""
			for _, o := range opts {
				if o.Text == sel {
					*d.text[e.Name] = o.Value
				}
			}
		}
	case "bool":
		ui.Checkbox(c.Key("v"), d.on[e.Name], e.Placeholder)
	default:
		in := ui.TextInput(c.Key("v"), d.text[e.Name]).Label(e.DisplayName).Placeholder(e.Placeholder)
		if e.Subtype == "password" {
			in.Password()
		}
		if e.MaxLength > 0 {
			in.MaxLength(e.MaxLength)
		}
	}
	if msg := d.errors[e.Name]; msg != "" {
		ui.Text(c, msg).FontSize(12).TextColor(t.Danger)
	} else if e.Help != "" {
		ui.Text(c, e.Help).FontSize(12).TextColor(t.TextMuted)
	}
}
