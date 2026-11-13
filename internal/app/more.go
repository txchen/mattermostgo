package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// SetPrefs saves preferences of the user, here at once and on the server.
func (a *App) SetPrefs(s *Server, prefs ...store.Preference) {
	me := s.Store.Me
	a.apply(s, store.PrefsChanged{Prefs: prefs})
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetPrefs(ctx, me, prefs) }, nil)
}

// DeletePrefs removes preferences of the user.
func (a *App) DeletePrefs(s *Server, prefs ...store.Preference) {
	me := s.Store.Me
	a.apply(s, store.PrefsChanged{Prefs: prefs, Deleted: true})
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.DeletePrefs(ctx, me, prefs) }, nil)
}

// SetSaved saves a post for later, or takes it out of the saved ones.
func (a *App) SetSaved(s *Server, id store.PostID, on bool) {
	p := store.Preference{Category: "flagged_post", Name: string(id), Value: "true"}
	if on {
		a.SetPrefs(s, p)
	} else {
		a.DeletePrefs(s, p)
	}
}

// SetPinned pins a post to its channel, or unpins it.
func (a *App) SetPinned(s *Server, id store.PostID, on bool) {
	if p := s.Store.Posts[id]; p != nil {
		next := *p
		next.Pinned = on
		a.apply(s, store.PostEdited{Post: next})
	}
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetPinned(ctx, id, on) }, nil)
}

// MarkPostUnread marks a post and those after it in its channel unread.
func (a *App) MarkPostUnread(s *Server, p *store.Post) {
	me, id, ch := s.Store.Me, p.ID, p.ChannelID
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.MarkUnread(ctx, me, id) }, func() {
		a.refreshChannel(s, ch, func() {
			if m := s.Store.Members[ch]; m != nil && s.Channel == ch {
				s.NewSince = m.LastViewedAt
			}
		})
	})
}

// Remind asks the server to remind the user of a post.
func (a *App) Remind(s *Server, id store.PostID, at time.Time) {
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.Remind(ctx, me, id, at) }, func() {
		a.showError("You will be reminded " + at.Format("Mon Jan 2 at 15:04"))
	})
}

// ForwardPost posts a link to a post in a channel, after a comment.
func (a *App) ForwardPost(s *Server, id store.PostID, to store.ChannelID, comment string) {
	msg := strings.TrimSpace(comment + "\n" + a.Permalink(s, id))
	a.Send(s, to, "", msg)
}

// Seen keeps posts the server listed, as search results, in the store,
// with their users.
func (a *App) Seen(s *Server, posts []store.Post) {
	ids := make([]store.UserID, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.UserID)
	}
	a.apply(s, store.PostsSeen{Posts: posts})
	a.askUsers(s, ids)
}

// postIDs lists the IDs of posts in their order.
func postIDs(posts []store.Post) []store.PostID {
	out := make([]store.PostID, len(posts))
	for i, p := range posts {
		out[i] = p.ID
	}
	return out
}

// ListPosts runs a request that lists posts, keeps them, and passes their
// IDs, newest first, to done.
func (a *App) ListPosts(s *Server, work func(ctx context.Context, c *mm.Client) (mm.PostResults, error), done func([]store.PostID)) {
	Call(a, s, work, func(r mm.PostResults) {
		a.Seen(s, r.Posts)
		done(postIDs(r.Posts))
	})
}

// LoadThreads loads the threads the user follows in the current team.
func (a *App) LoadThreads(s *Server) {
	me, team := s.Store.Me, s.Team
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.ThreadsLoaded, error) { return c.Threads(ctx, me, team) }, func(ev store.ThreadsLoaded) {
		a.apply(s, ev)
		var ids []store.UserID
		for _, t := range ev.Threads {
			ids = append(ids, t.Participants...)
		}
		for _, p := range ev.Posts {
			ids = append(ids, p.UserID)
		}
		a.askUsers(s, ids)
	})
}

// OpenThreads shows the Threads view in place of a channel.
func (a *App) OpenThreads(s *Server) {
	s.ThreadsView = true
	s.Thread = ""
	a.LoadThreads(s)
}

// SetFollowing follows a thread, or stops following it.
func (a *App) SetFollowing(s *Server, root store.PostID, on bool) {
	me, team := s.Store.Me, a.teamOfPost(s, root)
	if !on {
		a.apply(s, store.ThreadChanged{Thread: store.ThreadState{Root: root}})
	}
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetFollowing(ctx, me, team, root, on) }, func() {
		if on {
			a.LoadThreads(s)
		}
	})
}

func (a *App) teamOfPost(s *Server, id store.PostID) store.TeamID {
	if p := s.Store.Posts[id]; p != nil {
		if ch := s.Store.Channels[p.ChannelID]; ch != nil && ch.TeamID != "" {
			return ch.TeamID
		}
	}
	return s.Team
}

// readThread marks a followed thread read as it opens.
func (a *App) readThread(s *Server, root store.PostID) {
	if t := s.Store.Followed[root]; t != nil && (t.UnreadReplies > 0 || t.UnreadMentions > 0) {
		a.MarkThreadRead(s, root)
	}
}

// MarkThreadRead marks a followed thread read now.
func (a *App) MarkThreadRead(s *Server, root store.PostID) {
	t := s.Store.Followed[root]
	if t == nil {
		return
	}
	now := time.Now().UnixMilli()
	me, team := s.Store.Me, t.Team
	a.apply(s, store.ThreadRead{Root: root, LastViewedAt: now})
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.ReadThread(ctx, me, team, root, now) }, nil)
}

// Commands returns the slash commands of the current team, loading them
// the first time.
func (a *App) Commands(s *Server) []mm.AutocompleteCommand {
	if s.commands == nil {
		s.commands = map[store.TeamID][]mm.AutocompleteCommand{}
	}
	team := s.Team
	if cmds, ok := s.commands[team]; ok {
		return cmds
	}
	s.commands[team] = nil
	Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.AutocompleteCommand, error) {
		return c.AutocompleteCommands(ctx, team)
	},
		func(cmds []mm.AutocompleteCommand) { s.commands[team] = cmds })
	return nil
}

// RunCommand runs a slash command, as the webapp does instead of posting
// it: what it answers only for the user shows as a message.
func (a *App) RunCommand(s *Server, ch store.ChannelID, root store.PostID, text string) {
	team := s.Team
	if c := s.Store.Channels[ch]; c != nil && c.TeamID != "" {
		team = c.TeamID
	}
	Call(a, s, func(ctx context.Context, c *mm.Client) (mm.CommandResult, error) {
		return c.RunCommand(ctx, team, ch, root, text)
	},
		func(r mm.CommandResult) {
			switch {
			case r.GotoLocation != "":
				if !a.OpenPermalink(s, r.GotoLocation) {
					a.OpenURL(r.GotoLocation)
				}
			case r.Text != "" && r.ResponseType != "in_channel":
				// What only the user sees shows as a system post in the
				// channel, as the webapp makes it.
				now := time.Now().UnixMilli()
				a.apply(s, store.PostCreated{Post: store.Post{ID: store.PostID(fmt.Sprint("ephemeral-", now)), ChannelID: ch, RootID: root,
					Type: "system_ephemeral", Message: r.Text, CreateAt: now, Ephemeral: true}})
			}
		})
}

// CustomEmojis returns the server's custom emoji by name, loading them the
// first time.
func (a *App) CustomEmojis(s *Server) map[string]string {
	if s.emojis == nil {
		s.emojis = map[string]string{}
		Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.CustomEmoji, error) { return c.CustomEmojis(ctx) }, func(es []mm.CustomEmoji) {
			for _, e := range es {
				s.emojis[e.Name] = e.ID
			}
		})
	}
	return s.emojis
}

// OpenGroup opens the group message of the user and others.
func (a *App) OpenGroup(s *Server, users []store.UserID) {
	ids := append([]store.UserID{s.Store.Me}, users...)
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.Channel, error) { return c.GroupChannel(ctx, ids) }, func(ch store.Channel) {
		a.showConversation(s, ch.ID)
		a.refreshChannel(s, ch.ID, func() { a.OpenChannel(s, ch.ID) })
	})
}

// showConversation shows a closed DM or GM in the sidebar again.
func (a *App) showConversation(s *Server, id store.ChannelID) {
	ch := s.Store.Channels[id]
	if ch == nil || !s.Store.Hidden(ch) {
		return
	}
	switch ch.Type {
	case store.Direct:
		a.SetPrefs(s, store.Preference{Category: "direct_channel_show", Name: string(ch.DMPartner(s.Store.Me)), Value: "true"})
	case store.Group:
		a.SetPrefs(s, store.Preference{Category: "group_channel_show", Name: string(id), Value: "true"})
	}
}

// CloseConversation takes a DM or GM out of the sidebar.
func (a *App) CloseConversation(s *Server, id store.ChannelID) {
	ch := s.Store.Channels[id]
	if ch == nil {
		return
	}
	switch ch.Type {
	case store.Direct:
		a.SetPrefs(s, store.Preference{Category: "direct_channel_show", Name: string(ch.DMPartner(s.Store.Me)), Value: "false"})
	case store.Group:
		a.SetPrefs(s, store.Preference{Category: "group_channel_show", Name: string(id), Value: "false"})
	default:
		return
	}
	if s.Channel == id {
		s.Channel, s.Thread = "", ""
		a.openDefault(s)
	}
}

func (a *App) SetChannelAdmin(s *Server, id store.ChannelID, user store.UserID, admin bool, done func()) {
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetChannelAdmin(ctx, id, user, admin) }, done)
}

func (a *App) MakePublic(s *Server, id store.ChannelID) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.Channel, error) { return c.MakePublic(ctx, id) }, func(ch store.Channel) {
		a.apply(s, store.ChannelUpserted{Channel: ch})
	})
}

// RestoreChannel unarchives a channel and opens it.
func (a *App) RestoreChannel(s *Server, id store.ChannelID) {
	a.do(s, func(ctx context.Context, c *mm.Client) error { _, err := c.RestoreChannel(ctx, id); return err }, func() {
		a.refreshChannel(s, id, func() { a.OpenChannel(s, id) })
	})
}

// OpenArchived shows an archived channel, read only.
func (a *App) OpenArchived(s *Server, ch store.Channel) {
	a.apply(s, store.ChannelUpserted{Channel: ch, Member: &store.ChannelMember{ChannelID: ch.ID, MsgCount: ch.TotalMsgCount}})
	a.OpenChannel(s, ch.ID)
}

// SetNotify saves notification settings of the user.
func (a *App) SetNotify(s *Server, props map[string]string, done func()) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.User, error) { return c.SetNotify(ctx, props) }, func(u store.User) {
		a.apply(s, store.UsersLoaded{Users: []store.User{u}})
		if done != nil {
			done()
		}
	})
}

// SetChannelNotify saves the notification settings of the user in a channel.
func (a *App) SetChannelNotify(s *Server, id store.ChannelID, props map[string]string, done func()) {
	me := s.Store.Me
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetChannelNotify(ctx, me, id, props) }, func() {
		a.refreshChannel(s, id, done)
	})
}

// RefreshMe loads the user again, as after a new profile picture.
func (a *App) RefreshMe(s *Server) {
	Call(a, s, func(ctx context.Context, c *mm.Client) (store.User, error) { return c.Me(ctx) }, func(u store.User) {
		a.apply(s, store.UsersLoaded{Users: []store.User{u}})
	})
}

// PatchCategory changes a category of the current team, here at once and
// on the server.
func (a *App) PatchCategory(s *Server, cat *store.Category, fields map[string]any) {
	team, id := cat.TeamID, cat.ID
	if team == "" {
		team = s.Team
	}
	next := *cat
	for k, v := range fields {
		switch k {
		case "collapsed":
			next.Collapsed = v.(bool)
		case "muted":
			next.Muted = v.(bool)
		case "sorting":
			next.Sorting = v.(string)
		case "channel_ids":
			next.Channels = v.([]store.ChannelID)
		}
	}
	cats := make([]store.Category, 0, len(s.Store.Categories[team]))
	for _, c := range s.Store.Categories[team] {
		if c.ID == id {
			cats = append(cats, next)
		} else {
			cats = append(cats, *c)
		}
	}
	a.apply(s, store.CategoriesLoaded{Team: team, Categories: cats})
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.PatchCategory(ctx, team, id, fields) }, func() { a.refreshCategories(s, team) })
}

// MoveCategory puts a category before another, or last for "".
func (a *App) MoveCategory(s *Server, id, before string) {
	team := s.Team
	var order []string
	var moved store.Category
	cats := s.Store.Categories[team]
	for _, c := range cats {
		if c.ID == id {
			moved = *c
		}
	}
	var next []store.Category
	for _, c := range cats {
		if c.ID == id {
			continue
		}
		if c.ID == before {
			next = append(next, moved)
		}
		next = append(next, *c)
	}
	if before == "" {
		next = append(next, moved)
	}
	for _, c := range next {
		order = append(order, c.ID)
	}
	a.apply(s, store.CategoriesLoaded{Team: team, Categories: next})
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.SetCategoryOrder(ctx, team, order) }, func() { a.refreshCategories(s, team) })
}

// DropChannel moves a channel into a category, before another channel
// of it or last, which sorts the category by hand, as the webapp does.
func (a *App) DropChannel(s *Server, id store.ChannelID, category string, before store.ChannelID) {
	team := teamOf(s, id)
	var target *store.Category
	for _, c := range s.Store.Categories[team] {
		if c.ID == category {
			target = c
		}
	}
	if target == nil {
		return
	}
	var order []store.ChannelID
	for _, ch := range target.Channels {
		if ch == id {
			continue
		}
		if ch == before {
			order = append(order, id)
		}
		order = append(order, ch)
	}
	if before == "" || before == id {
		order = append(order, id)
	}
	if cur := a.Category(s, id); cur != nil && cur.ID != category {
		a.do(s, func(ctx context.Context, c *mm.Client) error {
			if err := c.MoveToCategory(ctx, team, id, category); err != nil {
				return err
			}
			return c.SetCategoryChannels(ctx, team, category, order)
		}, func() { a.refreshCategories(s, team) })
		return
	}
	a.PatchCategory(s, target, map[string]any{"channel_ids": order, "sorting": "manual"})
}

// ForgetTimeline drops the loaded posts of a channel, which load again as
// it opens.
func (a *App) ForgetTimeline(s *Server, id store.ChannelID) {
	a.apply(s, store.TimelineDropped{Channel: id})
}

// DoAction chooses a button, or with selected an option of a menu, of a
// post from an integration.
func (a *App) DoAction(s *Server, post store.PostID, action, selected string) {
	a.do(s, func(ctx context.Context, c *mm.Client) error { return c.DoAction(ctx, post, action, selected) }, nil)
}

// TakeDialog returns the interactive dialog waiting to show, once.
func (a *App) TakeDialog(s *Server) *store.Dialog {
	d := s.Store.Dialog
	if d != nil {
		a.apply(s, store.DialogTaken{})
	}
	return d
}

// SubmitDialog sends a dialog to its integration; done gets the errors it
// answered with by field, none when it took the dialog.
func (a *App) SubmitDialog(s *Server, d store.Dialog, values map[string]any, cancelled bool, done func(map[string]string)) {
	me, ch, team := s.Store.Me, s.Channel, s.Team
	Call(a, s, func(ctx context.Context, c *mm.Client) (map[string]string, error) {
		return c.SubmitDialog(ctx, d, me, ch, team, values, cancelled)
	}, done)
}
