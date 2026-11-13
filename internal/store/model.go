// Package store is the client's model of one server's data. Only the
// main thread reads or writes a Store, and only Apply changes it.
package store

import (
	"strings"
	"time"
)

type (
	UserID    string
	TeamID    string
	ChannelID string
	PostID    string
	FileID    string
)

type ChannelType string

const (
	Open    ChannelType = "O"
	Private ChannelType = "P"
	Direct  ChannelType = "D"
	Group   ChannelType = "G"
)

type User struct {
	ID        UserID
	Username  string
	FirstName string
	LastName  string
	Nickname  string
	IsBot     bool
	Deleted   bool
	// PictureAt changes when the user changes their avatar, so it keys the image cache.
	PictureAt int64
	// DesktopNotify is the user's own notification setting ("all",
	// "mention", "none"); the server sends it for the current user only.
	DesktopNotify string
}

func (u *User) FullName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

type Team struct {
	ID          TeamID
	Name        string
	DisplayName string
}

type Channel struct {
	ID          ChannelID
	TeamID      TeamID // empty for DMs and GMs
	Type        ChannelType
	Name        string
	DisplayName string
	Header      string
	Purpose     string
	LastPostAt  int64
	// TotalMsgCount counts the channel's root posts: replies count in
	// their threads, as with Collapsed Reply Threads (ADR 0005).
	TotalMsgCount int64
	Deleted       bool
	// MemberCount is 0 until the channel's stats load.
	MemberCount int64
}

// DMPartner returns the other user of a DM, or me for a DM with myself.
func (ch *Channel) DMPartner(me UserID) UserID {
	a, b, ok := strings.Cut(ch.Name, "__")
	if !ok || ch.Type != Direct {
		return ""
	}
	if UserID(a) == me {
		return UserID(b)
	}
	return UserID(a)
}

type ChannelMember struct {
	ChannelID    ChannelID
	LastViewedAt int64
	MsgCount     int64
	MentionCount int64
	Notify       NotifyProps
}

// NotifyProps are a channel member's notification settings. "default"
// means the user's account-wide setting applies.
type NotifyProps struct {
	Desktop    string // "default", "all", "mention", "none"
	MarkUnread string // "all", or "mention" for a muted channel
}

func (m *ChannelMember) Muted() bool { return m.Notify.MarkUnread == "mention" }

type CategoryType string

const (
	CategoryFavorites CategoryType = "favorites"
	CategoryChannels  CategoryType = "channels"
	CategoryDirect    CategoryType = "direct_messages"
	CategoryCustom    CategoryType = "custom"
)

type Category struct {
	ID          string
	TeamID      TeamID
	Type        CategoryType
	DisplayName string
	Sorting     string // "", "manual", "recent", "alpha"
	Muted       bool
	Collapsed   bool
	Channels    []ChannelID
}

type Status string

const (
	Online  Status = "online"
	Away    Status = "away"
	DND     Status = "dnd"
	Offline Status = "offline"
)

type PostState uint8

const (
	Sent PostState = iota
	Sending
	Failed
)

type Post struct {
	ID        PostID
	ChannelID ChannelID
	UserID    UserID
	RootID    PostID // empty for a root post
	CreateAt  int64
	EditAt    int64
	Message   string
	Type      string // "" for a user's post; "system_..." for system posts
	Props     PostProps
	Files     []FileInfo
	Reactions []Reaction
	// ReplyCount, LastReplyAt and Participants describe a root post's
	// thread; Participants are the users who replied, latest last.
	// Embed is the preview of the post's first link, if the server made one.
	Embed        *Embed
	Attachments  []Attachment
	ReplyCount   int64
	LastReplyAt  int64
	Participants []UserID
	// PendingID is the client's ID of a post it sent, echoed by the server.
	PendingID string
	State     PostState
}

// PostProps holds the post props the client shows.
type PostProps struct {
	OverrideUsername string
	// OverrideIconURL is the picture a webhook or bot post shows instead
	// of its user's.
	OverrideIconURL string
	FromBot         bool
}

// Image is a picture on the web that a post shows, with its size in
// pixels when the server knows it.
type Image struct {
	URL           string
	Width, Height int
}

// Embed is a link's preview: a page's OpenGraph card, or with ImageOnly
// the image the link points to.
type Embed struct {
	URL                          string
	SiteName, Title, Description string
	Image                        *Image
	ImageOnly                    bool
}

// Attachment is a message attachment, as webhooks and bots send them.
type Attachment struct {
	Fallback, Color, Pretext           string
	AuthorName, AuthorLink, AuthorIcon string
	Title, TitleLink, Text             string
	Fields                             []AttachmentField
	ImageURL, ThumbURL                 string
	Footer, FooterIcon                 string
}

type AttachmentField struct {
	Title, Value string
	Short        bool
}

func (p *Post) IsSystem() bool { return strings.HasPrefix(p.Type, "system_") }

type Reaction struct {
	UserID    UserID
	EmojiName string
	CreateAt  int64
}

type FileInfo struct {
	ID         FileID
	Name       string
	Extension  string
	MimeType   string
	Size       int64
	Width      int
	Height     int
	HasPreview bool
}

func (f *FileInfo) IsImage() bool { return strings.HasPrefix(f.MimeType, "image/") }

// Timeline is the loaded part of a channel's or a thread's posts.
type Timeline struct {
	Order    []PostID // oldest first
	HasOlder bool
	Loading  bool
}

// Conn is the state of a server's WebSocket.
type Conn interface{ isConn() }

type ConnConnecting struct{}
type ConnOnline struct{}
type ConnOffline struct {
	Retry time.Time
	Err   string
}

func (ConnConnecting) isConn() {}
func (ConnOnline) isConn()     {}
func (ConnOffline) isConn()    {}
