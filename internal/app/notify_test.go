package app

import (
	"testing"

	"github.com/txchen/mattermostgo/internal/store"
)

// A new post notifies by the channel's setting, else the user's, and
// never for the user's own posts, muted channels or the channel in view.
func TestShouldNotify(t *testing.T) {
	me := &store.User{ID: "me", DesktopNotify: "mention"}
	open := &store.Channel{ID: "c", Type: store.Open}
	dm := &store.Channel{ID: "d", Type: store.Direct}
	post := &store.Post{ID: "p", UserID: "bob"}
	member := func(desktop, markUnread string) *store.ChannelMember {
		return &store.ChannelMember{Notify: store.NotifyProps{Desktop: desktop, MarkUnread: markUnread}}
	}
	cases := []struct {
		name      string
		m         *store.ChannelMember
		ch        *store.Channel
		p         *store.Post
		mentioned bool
		viewing   bool
		want      bool
	}{
		{"mention by default", member("default", "all"), open, post, true, false, true},
		{"no mention by default", member("default", "all"), open, post, false, false, false},
		{"every post in a DM", member("default", "all"), dm, post, false, false, true},
		{"channel set to all", member("all", "all"), open, post, false, false, true},
		{"channel set to none", member("none", "all"), open, post, true, false, false},
		{"muted", member("default", "mention"), open, post, true, false, false},
		{"in view", member("all", "all"), open, post, true, true, false},
		{"own post", member("all", "all"), open, &store.Post{UserID: "me"}, true, false, false},
		{"system post", member("all", "all"), open, &store.Post{UserID: "bob", Type: "system_join_channel"}, false, false, false},
	}
	for _, tc := range cases {
		if got := shouldNotify(me, tc.m, tc.ch, tc.p, tc.mentioned, tc.viewing); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

// The badge counts mentions across servers, and shows a dot for unread
// posts in channels that are not muted.
func TestBadge(t *testing.T) {
	a, b := store.New(), store.New()
	a.Apply(store.Loaded{Me: store.User{ID: "me"},
		Channels: []store.Channel{{ID: "x", TotalMsgCount: 5}, {ID: "muted", TotalMsgCount: 5}},
		Members:  []store.ChannelMember{{ChannelID: "x", MsgCount: 5, MentionCount: 2}, {ChannelID: "muted", MsgCount: 1, Notify: store.NotifyProps{MarkUnread: "mention"}}}})
	if m, u := badge([]*store.Store{a}); m != 2 || u {
		t.Errorf("mentions %d, unread %v", m, u)
	}
	b.Apply(store.Loaded{Me: store.User{ID: "me"}, Channels: []store.Channel{{ID: "y", TotalMsgCount: 3}}, Members: []store.ChannelMember{{ChannelID: "y", MsgCount: 1}}})
	if m, u := badge([]*store.Store{a, b}); m != 2 || !u {
		t.Errorf("mentions %d, unread %v", m, u)
	}
}
