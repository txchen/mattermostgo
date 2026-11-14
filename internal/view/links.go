package view

import (
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/markdown"
)

// markdownOptions are the Markdown view's options for a server: mentions
// and channel links open in the app, other links in the browser.
func (v *View) markdownOptions(s *app.Server) *mdview.Options {
	o := v.md[s.Config.ID]
	if o == nil {
		o = &mdview.Options{Extensions: parser.GFM, Theme: &mdview.Theme{FontSize: 15}, OnLink: func(link string) { v.openLink(s, link) },
			LinkIcon: func(dest string, size float32) (mdview.InlineObject, bool) { return v.customEmoji(s, dest, size) }}
		v.md[s.Config.ID] = o
	}
	return o
}

func (v *View) openLink(s *app.Server, link string) {
	v.textClick = textClick{}
	switch {
	case strings.HasPrefix(link, markdown.MentionScheme):
		name, _ := url.PathUnescape(strings.TrimPrefix(link, markdown.MentionScheme))
		for id, u := range s.Store.Users {
			if u.Username == name && id != s.Store.Me {
				v.App.OpenDM(s, id)
				return
			}
		}
	case strings.HasPrefix(link, markdown.ChannelScheme):
		name, _ := url.PathUnescape(strings.TrimPrefix(link, markdown.ChannelScheme))
		for id, ch := range s.Store.Channels {
			if ch.Name == name && (ch.TeamID == s.Team || ch.TeamID == "") {
				v.App.OpenChannel(s, id)
				return
			}
		}
	default:
		if v.App.OpenPermalink(s, link) {
			return
		}
		if ui.SafeToOpen(link) {
			v.App.OpenURL(link)
		}
	}
}
