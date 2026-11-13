package store

import (
	"slices"
	"sort"
	"strings"
	"time"
)

// KeptTimelines is how many channels keep their loaded posts: the open
// channel and the most recently viewed others.
const KeptTimelines = 11

// TypingFor is how long a typing indicator shows after the last typing event.
const TypingFor = 5 * time.Second

type Store struct {
	Me         UserID
	Teams      map[TeamID]*Team
	TeamOrder  []TeamID
	Channels   map[ChannelID]*Channel
	Members    map[ChannelID]*ChannelMember
	Categories map[TeamID][]*Category
	Users      map[UserID]*User
	Statuses   map[UserID]Status
	Posts      map[PostID]*Post
	Timelines  map[ChannelID]*Timeline
	Threads    map[PostID]*Timeline
	Typing     map[ChannelID]map[UserID]time.Time
	// Prefs are the user's preferences on the server, by category and
	// name, as flagged_post for saved posts.
	Prefs map[string]map[string]string
	// Followed are the threads the user follows, for the Threads view.
	Followed map[PostID]*ThreadState
	Conn     Conn
	// recent lists the channels with a timeline, most recently loaded first.
	recent []ChannelID
}

func New() *Store {
	return &Store{
		Teams:      map[TeamID]*Team{},
		Channels:   map[ChannelID]*Channel{},
		Members:    map[ChannelID]*ChannelMember{},
		Categories: map[TeamID][]*Category{},
		Users:      map[UserID]*User{},
		Statuses:   map[UserID]Status{},
		Posts:      map[PostID]*Post{},
		Timelines:  map[ChannelID]*Timeline{},
		Threads:    map[PostID]*Timeline{},
		Typing:     map[ChannelID]map[UserID]time.Time{},
		Prefs:      map[string]map[string]string{},
		Followed:   map[PostID]*ThreadState{},
		Conn:       ConnConnecting{},
	}
}

// Apply is the reducer: the only function that changes a store.
func (s *Store) Apply(ev Event) {
	switch ev := ev.(type) {
	case Loaded:
		s.loaded(ev)
	case CategoriesLoaded:
		cats := make([]*Category, len(ev.Categories))
		for i := range ev.Categories {
			c := ev.Categories[i]
			cats[i] = &c
		}
		s.Categories[ev.Team] = cats
	case UsersLoaded:
		for _, u := range ev.Users {
			s.Users[u.ID] = &u
		}
	case StatusesChanged:
		for id, st := range ev.Statuses {
			s.Statuses[id] = st
		}
	case PostsLoading:
		s.timeline(ev.Channel).Loading = true
	case PostsFailed:
		if t := s.Timelines[ev.Channel]; t != nil {
			t.Loading = false
		}
	case PostsLoaded:
		s.postsLoaded(ev)
	case ThreadLoaded:
		t := &Timeline{}
		for _, p := range ev.Posts {
			s.dropPending(&p)
			s.Posts[p.ID] = &p
			t.Order = s.insert(t.Order, p.ID)
		}
		s.Threads[ev.Root] = t
	case PostSending:
		if s.confirmed(ev.Post.PendingID) {
			return
		}
		p := ev.Post
		p.ID = PostID(p.PendingID)
		p.State = Sending
		s.place(&p)
	case PostFailed:
		if p := s.Posts[PostID(ev.PendingID)]; p != nil && p.State == Sending {
			p.State = Failed
		}
	case PostCreated:
		s.postCreated(ev)
	case PostEdited:
		if old := s.Posts[ev.Post.ID]; old != nil {
			p := ev.Post
			s.Posts[p.ID] = &p
		}
	case PostDeleted:
		s.deletePost(ev.ID)
	case ReactionAdded:
		if p := s.Posts[ev.Post]; p != nil && !hasReaction(p, ev.Reaction.UserID, ev.Reaction.EmojiName) {
			p.Reactions = append(p.Reactions, ev.Reaction)
		}
	case ReactionRemoved:
		if p := s.Posts[ev.Post]; p != nil {
			p.Reactions = slices.DeleteFunc(p.Reactions, func(r Reaction) bool {
				return r.UserID == ev.User && r.EmojiName == ev.EmojiName
			})
		}
	case ChannelsViewed:
		for id, at := range ev.Times {
			m, ch := s.Members[id], s.Channels[id]
			if m == nil || ch == nil {
				continue
			}
			m.LastViewedAt = max(m.LastViewedAt, at)
			m.MsgCount = ch.TotalMsgCount
			m.MentionCount = 0
		}
	case MemberUpdated:
		if _, ok := s.Channels[ev.Member.ChannelID]; ok {
			m := ev.Member
			s.Members[m.ChannelID] = &m
		}
	case ChannelUpserted:
		ch := ev.Channel
		if old := s.Channels[ch.ID]; old != nil && ch.MemberCount == 0 {
			ch.MemberCount = old.MemberCount
		}
		s.Channels[ch.ID] = &ch
		if ev.Member != nil {
			m := *ev.Member
			s.Members[ch.ID] = &m
		}
	case ChannelRemoved:
		s.removeChannel(ev.Channel)
	case Typing:
		typers := s.Typing[ev.Channel]
		if typers == nil {
			typers = map[UserID]time.Time{}
			s.Typing[ev.Channel] = typers
		}
		typers[ev.User] = ev.At.Add(TypingFor)
		for u, until := range typers {
			if until.Before(ev.At) {
				delete(typers, u)
			}
		}
	case ConnChanged:
		s.Conn = ev.Conn
	case PostsSeen:
		for _, p := range ev.Posts {
			if s.Posts[p.ID] == nil {
				p := p
				s.Posts[p.ID] = &p
			}
		}
	case PrefsChanged:
		for _, p := range ev.Prefs {
			cat := s.Prefs[p.Category]
			if cat == nil {
				cat = map[string]string{}
				s.Prefs[p.Category] = cat
			}
			if ev.Deleted {
				delete(cat, p.Name)
			} else {
				cat[p.Name] = p.Value
			}
		}
	case ThreadsLoaded:
		for id, t := range s.Followed {
			if t.Team == ev.Team || t.Team == "" {
				delete(s.Followed, id)
			}
		}
		for _, p := range ev.Posts {
			if s.Posts[p.ID] == nil {
				p := p
				s.Posts[p.ID] = &p
			}
		}
		for _, t := range ev.Threads {
			t := t
			s.Followed[t.Root] = &t
		}
	case ThreadChanged:
		if !ev.Thread.Following {
			delete(s.Followed, ev.Thread.Root)
			break
		}
		t := ev.Thread
		s.Followed[t.Root] = &t
	case ThreadRead:
		if t := s.Followed[ev.Root]; t != nil {
			t.LastViewedAt, t.UnreadReplies, t.UnreadMentions = ev.LastViewedAt, ev.UnreadReplies, ev.UnreadMentions
		}
	case ChannelStats:
		if ch := s.Channels[ev.Channel]; ch != nil {
			ch.MemberCount = ev.Members
		}
	}
}

func (s *Store) loaded(ev Loaded) {
	s.Me = ev.Me.ID
	me := ev.Me
	s.Users[me.ID] = &me
	s.Teams = map[TeamID]*Team{}
	s.TeamOrder = s.TeamOrder[:0]
	for _, t := range ev.Teams {
		s.Teams[t.ID] = &t
		s.TeamOrder = append(s.TeamOrder, t.ID)
	}
	s.Channels = map[ChannelID]*Channel{}
	for _, ch := range ev.Channels {
		s.Channels[ch.ID] = &ch
	}
	if ev.Prefs != nil {
		s.Prefs = map[string]map[string]string{}
		s.Apply(PrefsChanged{Prefs: ev.Prefs})
	}
	s.Members = map[ChannelID]*ChannelMember{}
	for _, m := range ev.Members {
		if _, ok := s.Channels[m.ChannelID]; ok {
			s.Members[m.ChannelID] = &m
		}
	}
	for id := range s.Timelines {
		if _, ok := s.Channels[id]; !ok {
			s.dropTimeline(id)
		}
	}
	for team := range s.Categories {
		if _, ok := s.Teams[team]; !ok {
			delete(s.Categories, team)
		}
	}
}

func (s *Store) timeline(id ChannelID) *Timeline {
	t := s.Timelines[id]
	if t == nil {
		t = &Timeline{}
		s.Timelines[id] = t
		s.touch(id)
	}
	return t
}

// touch makes a channel the most recently loaded and drops the timelines
// beyond KeptTimelines.
func (s *Store) touch(id ChannelID) {
	s.recent = slices.DeleteFunc(s.recent, func(c ChannelID) bool { return c == id })
	s.recent = slices.Insert(s.recent, 0, id)
	for len(s.recent) > KeptTimelines {
		s.dropTimeline(s.recent[len(s.recent)-1])
	}
}

func (s *Store) dropTimeline(id ChannelID) {
	delete(s.Timelines, id)
	s.recent = slices.DeleteFunc(s.recent, func(c ChannelID) bool { return c == id })
	for pid, p := range s.Posts {
		if p.ChannelID == id {
			delete(s.Posts, pid)
			delete(s.Threads, pid)
		}
	}
}

func (s *Store) postsLoaded(ev PostsLoaded) {
	t := s.timeline(ev.Channel)
	t.Loading = false
	if !ev.Older {
		s.touch(ev.Channel)
		// Keep posts still being sent; drop the rest of the old page.
		kept := t.Order[:0]
		for _, id := range t.Order {
			if p := s.Posts[id]; p != nil && p.State != Sent {
				kept = append(kept, id)
			} else if !s.inThread(id) {
				delete(s.Posts, id)
			}
		}
		t.Order = kept
	}
	t.HasOlder = ev.HasOlder
	for _, p := range ev.Posts {
		if p.ChannelID != ev.Channel || p.RootID != "" {
			continue
		}
		s.dropPending(&p)
		s.Posts[p.ID] = &p
		t.Order = s.insert(t.Order, p.ID)
	}
}

func (s *Store) inThread(id PostID) bool {
	for _, t := range s.Threads {
		if slices.Contains(t.Order, id) {
			return true
		}
	}
	return false
}

// place puts a post into the loaded timeline of its channel and thread.
// It reports whether the store keeps it.
func (s *Store) place(p *Post) bool {
	timeline, thread := s.Timelines[p.ChannelID], s.Threads[p.RootID]
	if p.RootID != "" {
		timeline = nil // a channel shows its root posts; replies are in threads
	}
	if timeline == nil && (p.RootID == "" || thread == nil) {
		return false
	}
	s.Posts[p.ID] = p
	if timeline != nil {
		timeline.Order = s.insert(timeline.Order, p.ID)
	}
	if p.RootID != "" && thread != nil {
		thread.Order = s.insert(thread.Order, p.ID)
	}
	return true
}

func (s *Store) postCreated(ev PostCreated) {
	p := ev.Post
	p.State = Sent
	if _, known := s.Posts[p.ID]; known {
		return
	}
	s.dropPending(&p)
	s.place(&p)
	if typers := s.Typing[p.ChannelID]; typers != nil {
		delete(typers, p.UserID)
	}
	ch, m := s.Channels[p.ChannelID], s.Members[p.ChannelID]
	if p.RootID != "" {
		if root := s.Posts[p.RootID]; root != nil {
			root.ReplyCount++
			root.LastReplyAt = max(root.LastReplyAt, p.CreateAt)
			root.Participants = append(slices.DeleteFunc(root.Participants, func(u UserID) bool { return u == p.UserID }), p.UserID)
		}
		if ch != nil {
			ch.LastPostAt = max(ch.LastPostAt, p.CreateAt)
		}
		return
	}
	if ch == nil {
		return
	}
	ch.TotalMsgCount++
	ch.LastPostAt = max(ch.LastPostAt, p.CreateAt)
	if m == nil {
		return
	}
	if p.UserID == s.Me {
		m.MsgCount = ch.TotalMsgCount
		m.MentionCount = 0
		m.LastViewedAt = max(m.LastViewedAt, p.CreateAt)
	} else if ev.Mentioned {
		m.MentionCount++
	}
}

// dropPending removes the pending post that a confirmed post replaces.
func (s *Store) dropPending(p *Post) {
	if p.PendingID == "" {
		return
	}
	if old := s.Posts[PostID(p.PendingID)]; old != nil && old.State != Sent {
		s.removeFromOrders(old.ID)
		delete(s.Posts, old.ID)
	}
}

func (s *Store) removeFromOrders(id PostID) {
	del := func(o []PostID) []PostID { return slices.DeleteFunc(o, func(x PostID) bool { return x == id }) }
	for _, t := range s.Timelines {
		t.Order = del(t.Order)
	}
	for _, t := range s.Threads {
		t.Order = del(t.Order)
	}
}

func (s *Store) deletePost(id PostID) {
	s.removeFromOrders(id)
	if t := s.Threads[id]; t != nil {
		for _, r := range slices.Clone(t.Order) {
			if r != id {
				s.removeFromOrders(r)
				delete(s.Posts, r)
			}
		}
		delete(s.Threads, id)
	}
	delete(s.Posts, id)
}

func (s *Store) removeChannel(id ChannelID) {
	s.dropTimeline(id)
	delete(s.Channels, id)
	delete(s.Members, id)
	delete(s.Typing, id)
	for _, cats := range s.Categories {
		for _, c := range cats {
			c.Channels = slices.DeleteFunc(c.Channels, func(x ChannelID) bool { return x == id })
		}
	}
}

// insert adds a post ID to an order sorted by creation time, once.
func (s *Store) insert(order []PostID, id PostID) []PostID {
	if slices.Contains(order, id) {
		return order
	}
	p := s.Posts[id]
	i := sort.Search(len(order), func(i int) bool {
		q := s.Posts[order[i]]
		if q == nil {
			return false
		}
		if q.CreateAt != p.CreateAt {
			return q.CreateAt > p.CreateAt
		}
		return strings.Compare(string(q.ID), string(p.ID)) > 0
	})
	return slices.Insert(order, i, id)
}

// confirmed reports whether the server's post for a pending ID is loaded.
func (s *Store) confirmed(pending string) bool {
	for _, p := range s.Posts {
		if p.State == Sent && p.PendingID == pending {
			return true
		}
	}
	return false
}

func hasReaction(p *Post, u UserID, emoji string) bool {
	return slices.ContainsFunc(p.Reactions, func(r Reaction) bool { return r.UserID == u && r.EmojiName == emoji })
}

// Unread reports whether a channel has posts the user has not viewed.
func (s *Store) Unread(id ChannelID) bool {
	ch, m := s.Channels[id], s.Members[id]
	return ch != nil && m != nil && ch.TotalMsgCount > m.MsgCount
}

// Mentions is the number of unread posts mentioning the user in a channel.
func (s *Store) Mentions(id ChannelID) int64 {
	if m := s.Members[id]; m != nil {
		return max(m.MentionCount, 0)
	}
	return 0
}

// Typers returns who is typing in a channel at now, and when the first of
// them stops showing.
func (s *Store) Typers(id ChannelID, now time.Time) (users []UserID, next time.Time) {
	for u, until := range s.Typing[id] {
		if until.After(now) {
			users = append(users, u)
			if next.IsZero() || until.Before(next) {
				next = until
			}
		}
	}
	slices.Sort(users)
	return users, next
}

// Pref returns a preference of the user, or "" when unset.
func (s *Store) Pref(category, name string) string { return s.Prefs[category][name] }

// Saved reports whether the user saved a post.
func (s *Store) Saved(id PostID) bool { return s.Pref("flagged_post", string(id)) == "true" }

// Hidden reports whether the user closed a DM or GM, which the sidebar
// then leaves out until a new post comes.
func (s *Store) Hidden(ch *Channel) bool {
	switch ch.Type {
	case Direct:
		return s.Pref("direct_channel_show", string(ch.DMPartner(s.Me))) == "false"
	case Group:
		return s.Pref("group_channel_show", string(ch.ID)) == "false"
	}
	return false
}

// UnreadThreads counts the followed threads of a team with unread
// replies, and the mentions in them.
func (s *Store) UnreadThreads(team TeamID) (threads, mentions int64) {
	for _, t := range s.Followed {
		if t.Team != team {
			continue
		}
		if t.UnreadReplies > 0 {
			threads++
		}
		mentions += t.UnreadMentions
	}
	return threads, mentions
}
