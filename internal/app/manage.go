package app

import (
	"context"
	"time"

	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// Call runs a request to the server off the main thread and passes its
// result to done on it, unless the session ended meanwhile. A failure
// shows as an error; done runs only on success.
func Call[T any](a *App, s *Server, work func(ctx context.Context, c *mm.Client) (T, error), done func(T)) {
	c := s.Client()
	if c == nil {
		return
	}
	st := s.Store
	goRun(a, func() (T, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return work(ctx, c)
	}, func(v T, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		if done != nil {
			done(v)
		}
	})
}

// do is Call for requests that return nothing.
func (a *App) do(s *Server, work func(ctx context.Context, c *mm.Client) error, done func()) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (struct{}, error) { return struct{}{}, work(ctx, c) }, func(struct{}) {
		if done != nil {
			done()
		}
	})
}

// refreshChannel loads a channel and the user's membership of it again,
// with the team's categories, as the WebSocket's channel events do.
func (a *App) refreshChannel(s *Server, id store.ChannelID, then func()) {
	type result struct {
		ev   store.ChannelUpserted
		cats []store.Category
	}
	Call(a, s, func(ctx context.Context, c *mm.Client) (result, error) {
		ch, err := c.Channel(ctx, id)
		if err != nil {
			return result{}, err
		}
		m, err := c.Member(ctx, id)
		if err != nil {
			return result{}, err
		}
		r := result{ev: store.ChannelUpserted{Channel: ch, Member: &m}}
		if ch.TeamID != "" {
			r.cats, _ = c.Categories(ctx, ch.TeamID)
		}
		return r, nil
	}, func(r result) {
		a.apply(s, r.ev)
		if r.cats != nil {
			a.apply(s, store.CategoriesLoaded{Team: r.ev.Channel.TeamID, Categories: r.cats})
		}
		if then != nil {
			then()
		}
	})
}

func (a *App) refreshCategories(s *Server, team store.TeamID) {
	Call(a, s, func(ctx context.Context, c *mm.Client) ([]store.Category, error) { return c.Categories(ctx, team) }, func(cats []store.Category) {
		a.apply(s, store.CategoriesLoaded{Team: team, Categories: cats})
	})
}

// MarkUnread marks a channel's last post unread, as the webapp's sidebar
// menu does.
func (a *App) MarkUnread(s *Server, id store.ChannelID) {
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error {
		last, err := c.LastPost(ctx, id)
		if err != nil || last == "" {
			return err
		}
		return c.MarkUnread(ctx, me, last)
	}, func() {
		a.refreshChannel(s, id, func() {
			if m := s.Store.Members[id]; m != nil && s.Channel == id {
				s.NewSince = m.LastViewedAt
			}
		})
	})
}

// SetMuted mutes a channel or unmutes it.
func (a *App) SetMuted(s *Server, id store.ChannelID, muted bool) {
	mark := "all"
	if muted {
		mark = "mention"
	}
	if m := s.Store.Members[id]; m != nil {
		next := *m
		next.Notify.MarkUnread = mark
		a.apply(s, store.MemberUpdated{Member: next})
	}
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetMarkUnread(ctx, me, id, mark) }, nil)
}

// teamOf is the team whose sidebar shows a channel: its own, or the
// current team for a DM or GM.
func teamOf(s *Server, id store.ChannelID) store.TeamID {
	if ch := s.Store.Channels[id]; ch != nil && ch.TeamID != "" {
		return ch.TeamID
	}
	return s.Team
}

// MoveToCategory files a channel under a sidebar category.
func (a *App) MoveToCategory(s *Server, id store.ChannelID, category string) {
	team := teamOf(s, id)
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.MoveToCategory(ctx, team, id, category) }, func() { a.refreshCategories(s, team) })
}

// Category returns the sidebar category of a channel in its team.
func (a *App) Category(s *Server, id store.ChannelID) *store.Category {
	cats := s.Store.Categories[teamOf(s, id)]
	for i := range cats {
		for _, ch := range cats[i].Channels {
			if ch == id {
				return cats[i]
			}
		}
	}
	return nil
}

// categoryOfType returns the team's category of a type, as Favorites.
func (a *App) categoryOfType(s *Server, team store.TeamID, t store.CategoryType) *store.Category {
	cats := s.Store.Categories[team]
	for i := range cats {
		if cats[i].Type == t {
			return cats[i]
		}
	}
	return nil
}

func (a *App) IsFavorite(s *Server, id store.ChannelID) bool {
	cat := a.Category(s, id)
	return cat != nil && cat.Type == store.CategoryFavorites
}

// SetFavorite moves a channel into Favorites, or back to the default
// category of its kind.
func (a *App) SetFavorite(s *Server, id store.ChannelID, on bool) {
	ch := s.Store.Channels[id]
	if ch == nil {
		return
	}
	kind := store.CategoryFavorites
	if !on {
		kind = store.CategoryChannels
		if ch.Type == store.Direct || ch.Type == store.Group {
			kind = store.CategoryDirect
		}
	}
	if cat := a.categoryOfType(s, teamOf(s, id), kind); cat != nil {
		a.MoveToCategory(s, id, cat.ID)
	}
}

// CreateCategory makes a sidebar category, with a channel moved into it
// when channel is set.
func (a *App) CreateCategory(s *Server, name string, channel store.ChannelID) {
	team := s.Team
	if channel != "" {
		team = teamOf(s, channel)
	}
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error {
		id, err := c.CreateCategory(ctx, me, team, name)
		if err != nil || channel == "" {
			return err
		}
		return c.MoveToCategory(ctx, team, channel, id)
	}, func() { a.refreshCategories(s, team) })
}

func (a *App) RenameCategory(s *Server, cat *store.Category, name string) {
	team, id := cat.TeamID, cat.ID
	if team == "" {
		team = s.Team
	}
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.RenameCategory(ctx, team, id, name) }, func() { a.refreshCategories(s, team) })
}

func (a *App) DeleteCategory(s *Server, cat *store.Category) {
	team, id := cat.TeamID, cat.ID
	if team == "" {
		team = s.Team
	}
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.DeleteCategory(ctx, team, id) }, func() { a.refreshCategories(s, team) })
}

// AddMembers adds users to a channel.
func (a *App) AddMembers(s *Server, id store.ChannelID, users []store.UserID, done func()) {
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.AddMembers(ctx, id, users) }, func() {
		a.refreshStats(s, id)
		if done != nil {
			done()
		}
	})
}

// RemoveMember takes another user out of a channel.
func (a *App) RemoveMember(s *Server, id store.ChannelID, user store.UserID, done func()) {
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.RemoveMember(ctx, id, user) }, func() {
		a.refreshStats(s, id)
		if done != nil {
			done()
		}
	})
}

func (a *App) refreshStats(s *Server, id store.ChannelID) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (int64, error) { return c.MemberCount(ctx, id) }, func(n int64) {
		a.apply(s, store.ChannelStats{Channel: id, Members: n})
	})
}

// Members loads who is in a channel, with their users.
func (a *App) Members(s *Server, id store.ChannelID, done func([]mm.MemberInfo)) {
	Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.MemberInfo, error) { return c.Members(ctx, id) }, func(ms []mm.MemberInfo) {
		ids := make([]store.UserID, len(ms))
		for i, m := range ms {
			ids[i] = m.User
		}
		a.askUsers(s, ids)
		done(ms)
	})
}

// LeaveChannel takes the user out of a channel.
func (a *App) LeaveChannel(s *Server, id store.ChannelID) {
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.RemoveMember(ctx, id, me) }, func() {
		a.apply(s, store.ChannelRemoved{Channel: id})
	})
}

// ArchiveChannel archives a channel for everyone.
func (a *App) ArchiveChannel(s *Server, id store.ChannelID) {
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.ArchiveChannel(ctx, id) }, func() {
		a.apply(s, store.ChannelRemoved{Channel: id})
	})
}

// PatchChannel changes a channel's name, header or purpose.
func (a *App) PatchChannel(s *Server, id store.ChannelID, p mm.ChannelPatch, done func()) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.Channel, error) { return c.PatchChannel(ctx, id, p) }, func(ch store.Channel) {
		a.apply(s, store.ChannelUpserted{Channel: ch})
		if done != nil {
			done()
		}
	})
}

func (a *App) MakePrivate(s *Server, id store.ChannelID) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.Channel, error) { return c.MakePrivate(ctx, id) }, func(ch store.Channel) {
		a.apply(s, store.ChannelUpserted{Channel: ch})
	})
}

// CreateChannel creates a channel in the current team and opens it.
func (a *App) CreateChannel(s *Server, n mm.NewChannel, done func(error)) {
	n.TeamID = s.Team
	c := s.Client()
	if c == nil {
		return
	}
	st := s.Store
	goRun(a, func() (store.Channel, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.CreateChannel(ctx, n)
	}, func(ch store.Channel, err error) {
		if s.Store != st {
			return
		}
		done(err)
		if err == nil {
			a.refreshChannel(s, ch.ID, func() { a.OpenChannel(s, ch.ID) })
		}
	})
}

// PublicChannels lists the team's public channels the user is not in.
func (a *App) PublicChannels(s *Server, done func([]store.Channel)) {
	team := s.Team
	Call(a, s, func(ctx context.Context, c *mm.Client) ([]store.Channel, error) { return c.PublicChannels(ctx, team) }, func(chs []store.Channel) {
		out := chs[:0]
		for _, ch := range chs {
			if _, in := s.Store.Channels[ch.ID]; !in {
				out = append(out, ch)
			}
		}
		done(out)
	})
}

// JoinChannel joins a public channel and opens it.
func (a *App) JoinChannel(s *Server, id store.ChannelID) {
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.AddMembers(ctx, id, []store.UserID{me}) }, func() {
		a.refreshChannel(s, id, func() { a.OpenChannel(s, id) })
	})
}

// ChannelLink is the web address of a channel, as the webapp copies it.
func (a *App) ChannelLink(s *Server, id store.ChannelID) string {
	ch := s.Store.Channels[id]
	team := s.Store.Teams[teamOf(s, id)]
	if ch == nil || team == nil {
		return ""
	}
	path := "/channels/" + ch.Name
	switch ch.Type {
	case store.Direct:
		path = "/messages/@" + a.UserName(s, ch.DMPartner(s.Store.Me))
	case store.Group:
		path = "/messages/" + ch.Name
	}
	return s.URL.String() + "/" + team.Name + path
}

// SetStatus sets the status of the user, as online, away, DND until
// dndEnd (zero for until changed) or offline.
func (a *App) SetStatus(s *Server, status store.Status, dndEnd time.Time) {
	me := s.Store.Me
	a.apply(s, store.StatusesChanged{Statuses: map[store.UserID]store.Status{me: status}})
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetStatus(ctx, me, status, dndEnd) }, nil)
}

// SetCustomStatus sets the custom status of the user, or clears it when
// cs is nil.
func (a *App) SetCustomStatus(s *Server, cs *store.CustomStatus, duration string) {
	me := s.Me()
	if me == nil {
		return
	}
	u := *me
	u.CustomStatus = cs
	a.apply(s, store.UsersLoaded{Users: []store.User{u}})
	a.do(s, func(ctx context.Context, c *mm.Client) error {
		if cs == nil {
			return c.ClearCustomStatus(ctx)
		}
		return c.SetCustomStatus(ctx, *cs, duration)
	}, nil)
}

// PatchMe saves the profile of the user.
func (a *App) PatchMe(s *Server, p mm.UserPatch, done func()) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.User, error) { return c.PatchMe(ctx, p) }, func(u store.User) {
		a.apply(s, store.UsersLoaded{Users: []store.User{u}})
		if done != nil {
			done()
		}
	})
}
