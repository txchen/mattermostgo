package store

import "time"

// Event is a typed change to a store, made at the network boundary from an
// API response or a WebSocket message.
type Event interface{ isEvent() }

// Loaded replaces what the server says about the user, their teams and
// channels. It comes at login and after every reconnect.
type Loaded struct {
	Me       User
	Teams    []Team
	Channels []Channel
	Members  []ChannelMember
}

type CategoriesLoaded struct {
	Team       TeamID
	Categories []Category // in the server's order
}

type UsersLoaded struct{ Users []User }

type StatusesChanged struct{ Statuses map[UserID]Status }

type PostsLoading struct {
	Channel ChannelID
	Older   bool
}

// PostsLoaded brings a page of a channel's posts. A page that is not Older
// is the newest page and replaces the loaded timeline.
type PostsLoaded struct {
	Channel  ChannelID
	Posts    []Post
	Older    bool
	HasOlder bool
}

type PostsFailed struct{ Channel ChannelID }

type ThreadLoaded struct {
	Root  PostID
	Posts []Post // the root and its replies
}

// PostSending is a post the user sent, before the server confirms it.
type PostSending struct{ Post Post }

type PostFailed struct{ PendingID string }

// PostCreated is a new post confirmed by the server. Mentioned is true
// when it mentions the current user.
type PostCreated struct {
	Post      Post
	Mentioned bool
}

type PostEdited struct{ Post Post }

type PostDeleted struct {
	ID      PostID
	Channel ChannelID
}

type ReactionAdded struct {
	Post     PostID
	Reaction Reaction
}

type ReactionRemoved struct {
	Post      PostID
	User      UserID
	EmojiName string
}

// ChannelsViewed marks channels read up to the given times, on this
// device or another.
type ChannelsViewed struct{ Times map[ChannelID]int64 }

type MemberUpdated struct{ Member ChannelMember }

type ChannelUpserted struct {
	Channel Channel
	Member  *ChannelMember
}

type ChannelRemoved struct{ Channel ChannelID }

type Typing struct {
	Channel ChannelID
	User    UserID
	At      time.Time
}

type ConnChanged struct{ Conn Conn }

type ChannelStats struct {
	Channel ChannelID
	Members int64
}

func (Loaded) isEvent()           {}
func (CategoriesLoaded) isEvent() {}
func (UsersLoaded) isEvent()      {}
func (StatusesChanged) isEvent()  {}
func (PostsLoading) isEvent()     {}
func (PostsLoaded) isEvent()      {}
func (PostsFailed) isEvent()      {}
func (ThreadLoaded) isEvent()     {}
func (PostSending) isEvent()      {}
func (PostFailed) isEvent()       {}
func (PostCreated) isEvent()      {}
func (PostEdited) isEvent()       {}
func (PostDeleted) isEvent()      {}
func (ReactionAdded) isEvent()    {}
func (ReactionRemoved) isEvent()  {}
func (ChannelsViewed) isEvent()   {}
func (MemberUpdated) isEvent()    {}
func (ChannelUpserted) isEvent()  {}
func (ChannelRemoved) isEvent()   {}
func (Typing) isEvent()           {}
func (ConnChanged) isEvent()      {}
func (ChannelStats) isEvent()     {}
