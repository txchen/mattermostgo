package store

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

const me UserID = "me"

func loadedStore(channels ...ChannelID) *Store {
	s := New()
	ev := Loaded{Me: User{ID: me, Username: "me"}, Teams: []Team{{ID: "t1", Name: "team"}}}
	for _, id := range channels {
		ev.Channels = append(ev.Channels, Channel{ID: id, TeamID: "t1", Type: Open, TotalMsgCount: 10})
		ev.Members = append(ev.Members, ChannelMember{ChannelID: id, MsgCount: 10})
	}
	s.Apply(ev)
	return s
}

func post(id PostID, ch ChannelID, at int64) Post {
	return Post{ID: id, ChannelID: ch, UserID: "u1", CreateAt: at, Message: string(id)}
}

func order(s *Store, ch ChannelID) []PostID {
	if t := s.Timelines[ch]; t != nil {
		return t.Order
	}
	return nil
}

// A page of posts arrives newest first and shows oldest first.
func TestPostsLoadedSortsOldestFirst(t *testing.T) {
	s := loadedStore("c")
	s.Apply(PostsLoaded{Channel: "c", Posts: []Post{post("b", "c", 2), post("a", "c", 1), post("c", "c", 3)}, HasOlder: true})
	if got := order(s, "c"); !slices.Equal(got, []PostID{"a", "b", "c"}) {
		t.Errorf("order %v", got)
	}
	s.Apply(PostsLoaded{Channel: "c", Posts: []Post{post("z", "c", 0)}, Older: true, HasOlder: false})
	if got := order(s, "c"); !slices.Equal(got, []PostID{"z", "a", "b", "c"}) || s.Timelines["c"].HasOlder {
		t.Errorf("order %v, has older %v", got, s.Timelines["c"].HasOlder)
	}
}

// The server's post replaces the pending one with the same pending ID, and
// the server's echo of it over the WebSocket changes nothing.
func TestPendingPostReplaced(t *testing.T) {
	s := loadedStore("c")
	s.Apply(PostsLoaded{Channel: "c", Posts: []Post{post("a", "c", 1)}})
	s.Apply(PostSending{Post: Post{PendingID: "me:1", ChannelID: "c", UserID: me, CreateAt: 5}})
	if got := order(s, "c"); !slices.Equal(got, []PostID{"a", "me:1"}) || s.Posts["me:1"].State != Sending {
		t.Fatalf("order %v", got)
	}
	real := Post{ID: "p", PendingID: "me:1", ChannelID: "c", UserID: me, CreateAt: 6}
	s.Apply(PostCreated{Post: real})
	s.Apply(PostCreated{Post: real})
	if got := order(s, "c"); !slices.Equal(got, []PostID{"a", "p"}) {
		t.Errorf("order %v", got)
	}
	if ch := s.Channels["c"]; ch.TotalMsgCount != 11 || s.Unread("c") {
		t.Errorf("total %d, unread %v", ch.TotalMsgCount, s.Unread("c"))
	}
}

// A failed send stays in the timeline, and a retry sends it again.
func TestFailedPostRetry(t *testing.T) {
	s := loadedStore("c")
	s.Apply(PostsLoaded{Channel: "c"})
	p := Post{PendingID: "me:1", ChannelID: "c", UserID: me, CreateAt: 5}
	s.Apply(PostSending{Post: p})
	s.Apply(PostFailed{PendingID: "me:1"})
	if s.Posts["me:1"].State != Failed {
		t.Fatal("not failed")
	}
	s.Apply(PostSending{Post: p})
	if got := order(s, "c"); len(got) != 1 || s.Posts["me:1"].State != Sending {
		t.Errorf("order %v state %v", got, s.Posts["me:1"].State)
	}
}

// Someone else's post makes a channel unread, a mention counts, and
// viewing it on another device clears both.
func TestUnreadAndMentions(t *testing.T) {
	s := loadedStore("c", "d")
	s.Apply(PostCreated{Post: post("x", "d", 1), Mentioned: true})
	s.Apply(PostCreated{Post: post("y", "d", 2)})
	if !s.Unread("d") || s.Mentions("d") != 1 || s.Unread("c") {
		t.Errorf("unread %v mentions %d", s.Unread("d"), s.Mentions("d"))
	}
	if _, kept := s.Posts["x"]; kept {
		t.Error("kept a post of a channel never opened")
	}
	s.Apply(ChannelsViewed{Times: map[ChannelID]int64{"d": 3}})
	if s.Unread("d") || s.Mentions("d") != 0 {
		t.Errorf("still unread after viewing")
	}
}

// Deleting a root post takes its thread with it.
func TestDeleteRootDropsThread(t *testing.T) {
	s := loadedStore("c")
	reply := post("r", "c", 2)
	reply.RootID = "a"
	s.Apply(PostsLoaded{Channel: "c", Posts: []Post{post("a", "c", 1), reply}})
	s.Apply(ThreadLoaded{Root: "a", Posts: []Post{post("a", "c", 1), reply}})
	s.Apply(PostDeleted{ID: "a", Channel: "c"})
	if len(order(s, "c")) != 0 || s.Threads["a"] != nil || len(s.Posts) != 0 {
		t.Errorf("order %v, posts %d", order(s, "c"), len(s.Posts))
	}
}

// Typing shows for TypingFor after the last typing event, and a post by
// the typer ends it.
func TestTyping(t *testing.T) {
	s := loadedStore("c")
	at := time.Unix(100, 0)
	s.Apply(Typing{Channel: "c", User: "u1", At: at})
	if u, next := s.Typers("c", at.Add(time.Second)); len(u) != 1 || !next.Equal(at.Add(TypingFor)) {
		t.Errorf("typers %v next %v", u, next)
	}
	if u, _ := s.Typers("c", at.Add(TypingFor+time.Millisecond)); len(u) != 0 {
		t.Errorf("still typing: %v", u)
	}
	s.Apply(Typing{Channel: "c", User: "u1", At: at})
	s.Apply(PostCreated{Post: post("p", "c", 1)})
	if u, _ := s.Typers("c", at); len(u) != 0 {
		t.Errorf("typing after posting: %v", u)
	}
}

// Only KeptTimelines channels keep their posts, the least recent dropped.
func TestTimelineLRU(t *testing.T) {
	var ids []ChannelID
	for i := range KeptTimelines + 2 {
		ids = append(ids, ChannelID(fmt.Sprint("c", i)))
	}
	s := loadedStore(ids...)
	for i, id := range ids {
		s.Apply(PostsLoaded{Channel: id, Posts: []Post{post(PostID(fmt.Sprint("p", i)), id, 1)}})
	}
	if len(s.Timelines) != KeptTimelines || s.Timelines[ids[0]] != nil || s.Posts["p0"] != nil {
		t.Errorf("%d timelines, first kept %v", len(s.Timelines), s.Timelines[ids[0]] != nil)
	}
}

// universe is a fixed set of posts, so the same ID always means the same
// post, as it does on a server.
type universe struct {
	channels []ChannelID
	posts    []Post
}

func newUniverse(r *rand.Rand) universe {
	u := universe{channels: []ChannelID{"c0", "c1", "c2"}}
	for i := range 30 {
		p := post(PostID(fmt.Sprint("p", i)), u.channels[r.IntN(3)], int64(r.IntN(20)))
		if i > 3 && r.IntN(3) == 0 {
			root := u.posts[r.IntN(i)]
			if root.RootID == "" && root.ChannelID == p.ChannelID {
				p.RootID = root.ID
				p.CreateAt = root.CreateAt + 1 + int64(r.IntN(5))
			}
		}
		if r.IntN(4) == 0 {
			p.UserID = me
			p.PendingID = fmt.Sprint("me:", i)
		}
		u.posts = append(u.posts, p)
	}
	return u
}

func (u universe) event(r *rand.Rand) Event {
	p := u.posts[r.IntN(len(u.posts))]
	ch := u.channels[r.IntN(len(u.channels))]
	some := func() []Post {
		var ps []Post
		for _, q := range u.posts {
			if q.ChannelID == ch && r.IntN(2) == 0 {
				ps = append(ps, q)
			}
		}
		return ps
	}
	switch r.IntN(12) {
	case 0, 1:
		return PostsLoaded{Channel: ch, Posts: some(), Older: r.IntN(2) == 0, HasOlder: r.IntN(2) == 0}
	case 2:
		if p.PendingID != "" {
			q := p
			q.ID = ""
			return PostSending{Post: q}
		}
		return PostsLoading{Channel: ch}
	case 3:
		return PostFailed{PendingID: p.PendingID}
	case 4, 5:
		return PostCreated{Post: p, Mentioned: r.IntN(2) == 0}
	case 6:
		return PostDeleted{ID: p.ID, Channel: p.ChannelID}
	case 7:
		return ReactionAdded{Post: p.ID, Reaction: Reaction{UserID: "u1", EmojiName: "smile"}}
	case 8:
		return ReactionRemoved{Post: p.ID, User: "u1", EmojiName: "smile"}
	case 9:
		return ChannelsViewed{Times: map[ChannelID]int64{ch: int64(r.IntN(20))}}
	case 10:
		root := p
		if root.RootID != "" {
			root = u.find(root.RootID)
		}
		thread := []Post{root}
		for _, q := range u.posts {
			if q.RootID == root.ID {
				thread = append(thread, q)
			}
		}
		return ThreadLoaded{Root: root.ID, Posts: thread}
	default:
		if r.IntN(3) == 0 {
			return ChannelRemoved{Channel: ch}
		}
		return ChannelUpserted{Channel: Channel{ID: ch, TeamID: "t1", Type: Open, TotalMsgCount: 10}, Member: &ChannelMember{ChannelID: ch, MsgCount: 10}}
	}
}

func (u universe) find(id PostID) Post {
	for _, p := range u.posts {
		if p.ID == id {
			return p
		}
	}
	panic(id)
}

func checkInvariants(s *Store) error {
	used := map[PostID]bool{}
	before := func(a, b *Post) bool {
		if a.CreateAt != b.CreateAt {
			return a.CreateAt < b.CreateAt
		}
		return a.ID < b.ID
	}
	confirmed := map[string]bool{}
	for _, p := range s.Posts {
		if p.State == Sent && p.PendingID != "" {
			confirmed[p.PendingID] = true
		}
	}
	for id, p := range s.Posts {
		if p.State != Sent && confirmed[string(id)] {
			return fmt.Errorf("pending %s kept beside its confirmed post", id)
		}
	}
	for ch, t := range s.Timelines {
		for i, id := range t.Order {
			p := s.Posts[id]
			if p == nil {
				return fmt.Errorf("timeline %s: %s not in posts", ch, id)
			}
			if used[id] {
				return fmt.Errorf("timeline %s: %s twice", ch, id)
			}
			used[id] = true
			if p.ChannelID != ch {
				return fmt.Errorf("timeline %s holds %s of %s", ch, id, p.ChannelID)
			}
			if i > 0 && !before(s.Posts[t.Order[i-1]], p) {
				return fmt.Errorf("timeline %s out of order at %d: %v", ch, i, t.Order)
			}
		}
	}
	for root, t := range s.Threads {
		seen := map[PostID]bool{}
		for _, id := range t.Order {
			p := s.Posts[id]
			if p == nil || seen[id] {
				return fmt.Errorf("thread %s: %s missing or twice", root, id)
			}
			seen[id] = true
			used[id] = true
			if id != root && p.RootID != root {
				return fmt.Errorf("thread %s holds %s of root %q", root, id, p.RootID)
			}
		}
	}
	for id := range s.Posts {
		if !used[id] {
			return fmt.Errorf("post %s kept in no timeline or thread", id)
		}
	}
	for id, m := range s.Members {
		ch := s.Channels[id]
		if ch == nil {
			return fmt.Errorf("member of missing channel %s", id)
		}
		if m.MentionCount < 0 || m.MsgCount > ch.TotalMsgCount {
			return fmt.Errorf("channel %s: mentions %d, viewed %d of %d", id, m.MentionCount, m.MsgCount, ch.TotalMsgCount)
		}
	}
	if len(s.Timelines) > KeptTimelines {
		return fmt.Errorf("%d timelines", len(s.Timelines))
	}
	return nil
}

// Random sequences of events keep the store's invariants after every event.
func TestGeneratedSequences(t *testing.T) {
	for seed := range uint64(500) {
		r := rand.New(rand.NewPCG(seed, 7))
		u := newUniverse(r)
		s := loadedStore(u.channels...)
		var log []Event
		for range 200 {
			ev := u.event(r)
			log = append(log, ev)
			s.Apply(ev)
			if err := checkInvariants(s); err != nil {
				t.Fatalf("seed %d after %d events: %v\nlast: %#v", seed, len(log), err, ev)
			}
		}
	}
}
