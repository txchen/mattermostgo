package app

import "github.com/txchen/mattermostgo/internal/store"

// shouldNotify decides whether a new post shows a desktop notification,
// from the channel's and the user's settings, as the webapp decides.
// viewing is true when the user has the channel open in a focused window.
func shouldNotify(me *store.User, m *store.ChannelMember, ch *store.Channel, p *store.Post, mentioned, viewing bool) bool {
	if me == nil || m == nil || ch == nil || p.UserID == me.ID || p.IsSystem() || viewing || m.Muted() {
		return false
	}
	level := m.Notify.Desktop
	if level == "" || level == "default" {
		level = me.DesktopNotify
	}
	switch level {
	case "all":
		return true
	case "none":
		return false
	default: // "mention", and the server's default
		return mentioned || ch.Type == store.Direct
	}
}

// badge is what the Dock shows across servers: the mention count, or a
// dot for unread posts without mentions.
func badge(stores []*store.Store) (mentions int64, unread bool) {
	for _, s := range stores {
		for id := range s.Channels {
			m := s.Members[id]
			if m == nil {
				continue
			}
			mentions += s.Mentions(id)
			if !m.Muted() && s.Unread(id) {
				unread = true
			}
		}
	}
	return mentions, unread
}
