package view

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/markdown"
	"github.com/txchen/mattermostgo/internal/store"
)

// suggestions are the completions of the word at the composer's caret.
type suggestions struct {
	items []suggestion
	sel   int
	// start and end are the runes the chosen completion replaces.
	start, end int
	word       string
	dismissed  string
	// stale asks for the suggestions again, as users arrive.
	stale    bool
	searched map[string]bool
}

type suggestion struct {
	label, detail, insert string
}

// completion replaces a word of the composer, if it is still there.
type completion struct {
	start, end   int
	word, insert string
}

// apply returns text with the completion made, if its word is still there.
func (cp completion) apply(text string) (string, bool) {
	runes := []rune(text)
	if cp.end > len(runes) || string(runes[cp.start:cp.end]) != cp.word {
		return text, false
	}
	return string(runes[:cp.start]) + cp.insert + string(runes[cp.end:]), true
}

// attachment is a file uploaded for the next post.
type attachment struct {
	name string
	info *store.FileInfo // nil while it uploads
}

// Hooks are what the view asks of the desktop, swapped out in tests.
type Hooks struct {
	PickFiles      func(done func(paths []string))
	ClipboardImage func() []byte
}

func (v *View) composer(c *ui.Context, s *app.Server, ch store.ChannelID, root store.PostID) {
	t := c.Theme()
	k := draftKey{s.Config.ID, ch, root}
	draft := v.drafts[k]
	if draft == nil {
		draft = new(string)
		v.drafts[k] = draft
	}
	sug := v.suggest[k]
	if sug == nil {
		sug = &suggestions{}
		v.suggest[k] = sug
	}
	files := v.attachments[k]
	placeholder := "Reply to this thread…"
	if root == "" {
		placeholder = "Write to " + v.App.ChannelName(s, s.Store.Channels[ch])
	}
	box := ui.Column(c).Padding(0, 20, 20, 20).Gap(4)
	box.Children(func() {
		if len(sug.items) > 0 {
			v.suggestionList(c, sug, func(text string) { v.complete[k] = completion{sug.start, sug.end, sug.word, text} })
		}
		frame := ui.Column(c).Radius(4).BorderWidth(1).Background(t.Background)
		frame.Children(func() {
			if len(files) > 0 {
				ui.Row(c).Wrap().Gap(8).Padding(12, 16, 0, 16).Children(func() {
					for i, f := range files {
						v.attachmentChip(c, k, files, i, f)
					}
				})
			}
			in := ui.TextAreaBase(c.Key("composer"), draft).Lines(1, 12).FillWidth().Padding(13, 16, 6, 16).FontSize(15).
				Placeholder(placeholder).Label("Message").AutoFocus().
				OnTextKey(func(key ui.TextKey) bool { return v.composerKey(s, k, sug, key) }).
				OnPaste(func(text string) bool {
					if text != "" || v.Hooks.ClipboardImage == nil {
						return false
					}
					if png := v.Hooks.ClipboardImage(); png != nil {
						v.attach(s, k, []string{"image.png"}, [][]byte{png})
						return true
					}
					return false
				})
			// Keys only note their edits; they are made here, as edits of
			// the text, so that what was typed after the key stays. A text
			// the key set would replace it, as the app setting the value.
			text := *draft
			// edited is true when this build edited the text: the draft and
			// the caret read the text from before the edit until the next.
			edited := false
			if cp, ok := v.complete[k]; ok {
				delete(v.complete, k)
				if done, ok := cp.apply(text); ok {
					_, caret := in.TextSelection()
					in.ReplaceText(cp.start, cp.end, cp.insert)
					if caret > cp.end {
						at := caret + len([]rune(cp.insert)) - len([]rune(cp.word))
						in.SetTextSelection(at, at)
					}
					text = done
					edited = true
				}
				sug.items = nil
			}
			if sent, ok := v.sent[k]; ok {
				delete(v.sent, k)
				if strings.HasPrefix(text, sent) {
					in.ReplaceText(0, len([]rune(sent)), "")
					edited = true
				}
			}
			if text, ok := v.inserts[k]; ok {
				delete(v.inserts, k)
				_, caret := in.TextSelection()
				in.ReplaceText(caret, caret, text)
				in.Focus()
			}
			if in.Changed() {
				v.App.Typing(s, ch, root)
			}
			if _, caret := in.TextSelection(); !edited {
				v.updateSuggestions(s, sug, *draft, caret)
			} else {
				c.Invalidate()
			}
			if in.FocusWithin() {
				frame.BorderColor(t.TextMuted.Alpha(0.48))
			} else {
				frame.BorderColor(t.Border)
			}
			v.composerToolbar(c, s, k, in, *draft != "" || len(files) > 0)
		})
	})
	if paths := box.DroppedFiles(); len(paths) > 0 {
		v.attachPaths(s, k, paths)
	}
	if box.FileDragOver() {
		box.Background(colors(c).link.Alpha(0.06))
	}
}

func (v *View) attachmentChip(c *ui.Context, k draftKey, files []attachment, i int, f attachment) {
	t := c.Theme()
	ui.Row(c.Key(i)).Gap(10).Padding(8, 8, 8, 12).Radius(4).BorderWidth(1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		symbol(c, "doc", 18, t.TextMuted)
		ui.Column(c).Children(func() {
			ui.Text(c, f.name).FontSize(13).FontWeight(600).MaxLines(1).MaxWidth(200)
			status := "Uploading…"
			if f.info != nil {
				status = strings.ToUpper(f.info.Extension) + " " + size(f.info.Size)
			}
			ui.Text(c, status).FontSize(11).TextColor(t.TextMuted)
		})
		if iconButton(c, "xmark", "Remove "+f.name, 10, t.TextMuted, t.SurfaceHover).Clicked() {
			v.attachments[k] = slices.Delete(slices.Clone(files), i, i+1)
		}
	})
}

// formats are the composer's formatting buttons: Markdown put around the
// selection, or at the start of its line.
var formats = []struct {
	symbol, label, before, after string
	line                         bool
}{
	{"bold", "Bold", "**", "**", false},
	{"italic", "Italic", "_", "_", false},
	{"strikethrough", "Strikethrough", "~~", "~~", false},
	{"textformat.size", "Heading", "### ", "", true},
	{"link", "Link", "[", "](https://)", false},
	{"chevron.left.forwardslash.chevron.right", "Code", "`", "`", false},
	{"text.quote", "Quote", "> ", "", true},
	{"list.bullet", "Bulleted list", "- ", "", true},
	{"list.number", "Numbered list", "1. ", "", true},
}

func (v *View) composerToolbar(c *ui.Context, s *app.Server, k draftKey, in ui.Element, canSend bool) {
	t := c.Theme()
	p := colors(c)
	muted := t.TextMuted
	ui.Row(c).Padding(0, 8, 8, 8).Gap(0).AlignItems(ui.Center).Children(func() {
		shown := formats
		if k.root != "" {
			shown = formats[:6] // the RHS is narrow
		}
		for i, f := range shown {
			if i == 4 || i == 6 {
				ui.Box(c).Width(1).Height(16).Background(t.Border).Margin(0, 4)
			}
			if iconButton(c.Key(f.label), f.symbol, f.label, 14, muted, t.SurfaceHover).Clicked() {
				applyFormat(in, *v.drafts[k], f.before, f.after, f.line)
			}
		}
		ui.Spacer(c)
		if iconButton(c, "paperclip", "Attach files", 15, muted, t.SurfaceHover).Clicked() && v.Hooks.PickFiles != nil {
			v.Hooks.PickFiles(func(paths []string) { v.attachPaths(s, k, paths) })
		}
		emoji := iconButton(c, "face.smiling", "Emoji", 15, muted, t.SurfaceHover)
		if emoji.Clicked() {
			v.picker = emojiPicker{open: true, composer: k, insert: func(name string) { v.inserts[k] = ":" + name + ": " }}
		}
		if v.picker.open && v.picker.insert != nil && v.picker.composer == k {
			v.emojiPopover(c, emoji, s)
		}
		send := ui.ButtonBase(c).Size(44, 32).Radius(4).Center().Label("Send").Tooltip("Send").Margin(0, 0, 0, 4).Disabled(!canSend)
		col := muted.Alpha(0.5)
		if canSend {
			send.Background(p.buttonBg)
			col = ui.Hex("#ffffff")
		} else {
			send.Background(t.Surface)
		}
		send.Children(func() { symbol(c, "paperplane.fill", 15, col) })
		if send.Clicked() {
			v.send(s, k, *v.drafts[k])
			*v.drafts[k] = ""
			in.ReplaceText(0, 1<<30, "")
		}
	})
}

// applyFormat puts before and after around the selection of a text input,
// or before at the start of the selection's line.
func applyFormat(in ui.Element, value, before, after string, line bool) {
	start, end := in.TextSelection()
	text := []rune(value)
	start, end = min(start, len(text)), min(end, len(text))
	if line {
		for start > 0 && text[start-1] != '\n' {
			start--
		}
		in.ReplaceText(start, start, before)
		in.SetTextSelection(end+len([]rune(before)), end+len([]rune(before)))
	} else {
		sel := string(text[start:end])
		in.ReplaceText(start, end, before+sel+after)
		at := start + len([]rune(before))
		in.SetTextSelection(at, at+len([]rune(sel)))
	}
	in.Focus()
}

func (v *View) composerKey(s *app.Server, k draftKey, sug *suggestions, key ui.TextKey) bool {
	if len(sug.items) > 0 {
		switch {
		case key.Key == ui.KeyUp:
			sug.sel = (sug.sel + len(sug.items) - 1) % len(sug.items)
			return true
		case key.Key == ui.KeyDown:
			sug.sel = (sug.sel + 1) % len(sug.items)
			return true
		case (key.Key == ui.KeyEnter || key.Key == ui.KeyTab) && key.Mods == 0:
			v.complete[k] = completion{sug.start, sug.end, sug.word, sug.items[sug.sel].insert}
			sug.items = nil
			return true
		case key.Key == ui.KeyEscape:
			sug.dismissed, sug.items = sug.word, nil
			return true
		}
	}
	switch {
	case (key.Key == ui.KeyUp || key.Key == ui.KeyDown) && key.Mods == ui.Alt:
		// As in the webapp, Option+Up and Down change channels from the
		// composer too, where the text area would move the caret.
		d := 1
		if key.Key == ui.KeyUp {
			d = -1
		}
		v.App.Run(func() { v.moveChannel(s, d) })
		return true
	case key.Key == ui.KeyEnter && key.Mods == 0:
		// A completion chosen just before still waits for the next frame:
		// what is sent has it.
		text := key.Text()
		if cp, ok := v.complete[k]; ok {
			if done, ok := cp.apply(text); ok {
				text = done
			}
		}
		v.send(s, k, text)
		v.sent[k] = text
		return true
	case key.Key == ui.KeyUp && key.Mods == 0 && key.Text() == "":
		if p := v.lastOwnPost(s, k); p != nil {
			v.editing, v.editText = p.ID, p.Message
			return true
		}
	}
	return false
}

func (v *View) send(s *app.Server, k draftKey, text string) {
	files := v.attachments[k]
	var infos []store.FileInfo
	for _, f := range files {
		if f.info == nil {
			v.ShowError("Wait for the files to upload.")
			return
		}
		infos = append(infos, *f.info)
	}
	if len(infos) > 0 {
		v.App.SendFiles(s, k.channel, k.root, text, infos)
		delete(v.attachments, k)
	} else {
		v.App.Send(s, k.channel, k.root, text)
	}
	if l := v.lists[listKey{s.Config.ID, listID(k)}]; l != nil {
		l.ScrollToEnd()
	}
}

func listID(k draftKey) string {
	if k.root != "" {
		return "th:" + string(k.root)
	}
	return "ch:" + string(k.channel)
}

func (v *View) lastOwnPost(s *app.Server, k draftKey) *store.Post {
	var order []store.PostID
	if k.root != "" {
		if t := s.Store.Threads[k.root]; t != nil {
			order = t.Order
		}
	} else if t := s.Store.Timelines[k.channel]; t != nil {
		order = t.Order
	}
	for i := len(order) - 1; i >= 0; i-- {
		if p := s.Store.Posts[order[i]]; p.UserID == s.Store.Me && p.State == store.Sent && !p.IsSystem() {
			return p
		}
	}
	return nil
}

func (v *View) attachPaths(s *app.Server, k draftKey, paths []string) {
	var names []string
	var data [][]byte
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			v.ShowError(err.Error())
			continue
		}
		names = append(names, filepath.Base(p))
		data = append(data, b)
	}
	if len(names) > 0 {
		v.attach(s, k, names, data)
	}
}

func (v *View) attach(s *app.Server, k draftKey, names []string, data [][]byte) {
	start := len(v.attachments[k])
	for _, n := range names {
		v.attachments[k] = append(v.attachments[k], attachment{name: n})
	}
	v.App.Upload(s, k.channel, names, data, func(infos []store.FileInfo) {
		files := v.attachments[k]
		for i := range infos {
			if start+i < len(files) && files[start+i].name == names[i] {
				files[start+i].info = &infos[i]
			}
		}
	})
}

// updateSuggestions finds the word at the caret and what completes it:
// @ users, ~ channels and : emoji.
func (v *View) updateSuggestions(s *app.Server, sug *suggestions, text string, caret int) {
	runes := []rune(text)
	caret = min(caret, len(runes))
	start := caret
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	word := string(runes[start:caret])
	if word == sug.word && start == sug.start && !sug.stale {
		return
	}
	sel := sug.sel
	if word != sug.word {
		sel = 0
	}
	sug.word, sug.items, sug.sel, sug.start, sug.end, sug.stale = word, nil, sel, start, caret, false
	if word != sug.dismissed {
		sug.dismissed = "" // Escape dismisses one word, not the next
	}
	if len(word) < 2 || word == sug.dismissed {
		return
	}
	prefix := strings.ToLower(word[1:])
	switch word[0] {
	case '@':
		sug.items = v.userSuggestions(s, prefix)
		// Ask the server once per prefix: the users it finds join the
		// store, and the next frame suggests them.
		if len(sug.items) < 5 && !sug.searched[prefix] {
			if sug.searched == nil {
				sug.searched = map[string]bool{}
			}
			sug.searched[prefix] = true
			v.App.SearchUsers(s, prefix, func([]store.User) { sug.stale = true })
		}
	case '~':
		for id, ch := range s.Store.Channels {
			if ch.TeamID != s.Team || ch.Type == store.Direct || ch.Type == store.Group {
				continue
			}
			if strings.HasPrefix(ch.Name, prefix) || strings.HasPrefix(strings.ToLower(ch.DisplayName), prefix) {
				sug.items = append(sug.items, suggestion{label: ch.DisplayName, detail: "~" + ch.Name, insert: "~" + s.Store.Channels[id].Name + " "})
			}
		}
		slices.SortFunc(sug.items, func(a, b suggestion) int { return strings.Compare(a.detail, b.detail) })
	case ':':
		if len(prefix) < 2 {
			return
		}
		for _, name := range markdown.EmojiNames(prefix, 8) {
			e, _ := markdown.Emoji(name)
			sug.items = append(sug.items, suggestion{label: e + "  :" + name + ":", insert: ":" + name + ": "})
		}
	}
	sug.items = sug.items[:min(8, len(sug.items))]
	sug.sel = min(sug.sel, max(len(sug.items)-1, 0))
}

func (v *View) userSuggestions(s *app.Server, prefix string) []suggestion {
	var out []suggestion
	for _, u := range s.Store.Users {
		if u.Deleted {
			continue
		}
		if strings.HasPrefix(u.Username, prefix) || strings.HasPrefix(strings.ToLower(u.FirstName), prefix) || strings.HasPrefix(strings.ToLower(u.LastName), prefix) || strings.HasPrefix(strings.ToLower(u.Nickname), prefix) {
			out = append(out, suggestion{label: "@" + u.Username, detail: u.FullName(), insert: "@" + u.Username + " "})
		}
	}
	slices.SortFunc(out, func(a, b suggestion) int { return strings.Compare(a.label, b.label) })
	for _, special := range []string{"here", "channel", "all"} {
		if strings.HasPrefix(special, prefix) {
			out = append(out, suggestion{label: "@" + special, detail: "Notifies everyone in the channel", insert: "@" + special + " "})
		}
	}
	return out
}

func (v *View) suggestionList(c *ui.Context, sug *suggestions, choose func(string)) {
	t := c.Theme()
	ui.Column(c).Radius(8).BorderWidth(1).BorderColor(t.Border).Background(t.Background).Padding(4).Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.12)).Children(func() {
		for i, it := range sug.items {
			row := ui.ButtonBase(c.Key(i)).FillWidth().Padding(6, 10).Radius(4).Label(it.label)
			if i == sug.sel {
				row.Background(t.Selection)
			}
			row.Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, it.label).FontSize(13)
					if it.detail != "" {
						ui.Text(c, it.detail).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
					}
				})
			})
			if row.Clicked() {
				choose(it.insert)
			}
		}
	})
}
