// Package mm talks to a Mattermost server: the REST API v4 and the
// WebSocket. It is the only package that sees the server's JSON, which it
// turns into store values and events.
package mm

import (
	"github.com/txchen/mattermostgo/internal/store"
)

// The wire types hold the fields of the server's JSON the client reads.
// Field names follow server/public/model in mattermost/mattermost.

type wireUser struct {
	ID                string            `json:"id"`
	Username          string            `json:"username"`
	FirstName         string            `json:"first_name"`
	LastName          string            `json:"last_name"`
	Nickname          string            `json:"nickname"`
	IsBot             bool              `json:"is_bot"`
	DeleteAt          int64             `json:"delete_at"`
	LastPictureUpdate int64             `json:"last_picture_update"`
	NotifyProps       map[string]string `json:"notify_props"`
}

func (u wireUser) user() store.User {
	return store.User{
		ID: store.UserID(u.ID), Username: u.Username, FirstName: u.FirstName, LastName: u.LastName,
		Nickname: u.Nickname, IsBot: u.IsBot, Deleted: u.DeleteAt != 0, PictureAt: u.LastPictureUpdate,
		DesktopNotify: u.NotifyProps["desktop"],
	}
}

type wireTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	DeleteAt    int64  `json:"delete_at"`
}

type wireChannel struct {
	ID            string `json:"id"`
	TeamID        string `json:"team_id"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Header        string `json:"header"`
	Purpose       string `json:"purpose"`
	LastPostAt    int64  `json:"last_post_at"`
	TotalMsgCount int64  `json:"total_msg_count_root"`
	DeleteAt      int64  `json:"delete_at"`
}

func (c wireChannel) channel() store.Channel {
	return store.Channel{
		ID: store.ChannelID(c.ID), TeamID: store.TeamID(c.TeamID), Type: store.ChannelType(c.Type),
		Name: c.Name, DisplayName: c.DisplayName, Header: c.Header, Purpose: c.Purpose,
		LastPostAt: c.LastPostAt, TotalMsgCount: c.TotalMsgCount, Deleted: c.DeleteAt != 0,
	}
}

type wireMember struct {
	ChannelID    string            `json:"channel_id"`
	LastViewedAt int64             `json:"last_viewed_at"`
	MsgCount     int64             `json:"msg_count_root"`
	MentionCount int64             `json:"mention_count_root"`
	NotifyProps  map[string]string `json:"notify_props"`
}

func (m wireMember) member() store.ChannelMember {
	return store.ChannelMember{
		ChannelID: store.ChannelID(m.ChannelID), LastViewedAt: m.LastViewedAt,
		MsgCount: m.MsgCount, MentionCount: m.MentionCount,
		Notify: store.NotifyProps{Desktop: m.NotifyProps["desktop"], MarkUnread: m.NotifyProps["mark_unread"]},
	}
}

type wireCategory struct {
	ID          string   `json:"id"`
	TeamID      string   `json:"team_id"`
	Type        string   `json:"type"`
	DisplayName string   `json:"display_name"`
	Sorting     string   `json:"sorting"`
	Muted       bool     `json:"muted"`
	Collapsed   bool     `json:"collapsed"`
	ChannelIDs  []string `json:"channel_ids"`
}

func (c wireCategory) category() store.Category {
	cat := store.Category{
		ID: c.ID, TeamID: store.TeamID(c.TeamID), Type: store.CategoryType(c.Type),
		DisplayName: c.DisplayName, Sorting: c.Sorting, Muted: c.Muted, Collapsed: c.Collapsed,
	}
	for _, id := range c.ChannelIDs {
		cat.Channels = append(cat.Channels, store.ChannelID(id))
	}
	return cat
}

type wireCategories struct {
	Categories []wireCategory `json:"categories"`
	Order      []string       `json:"order"`
}

type wireFile struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Extension       string `json:"extension"`
	MimeType        string `json:"mime_type"`
	Size            int64  `json:"size"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	HasPreviewImage bool   `json:"has_preview_image"`
}

func (f wireFile) file() store.FileInfo {
	return store.FileInfo{
		ID: store.FileID(f.ID), Name: f.Name, Extension: f.Extension, MimeType: f.MimeType,
		Size: f.Size, Width: f.Width, Height: f.Height, HasPreview: f.HasPreviewImage,
	}
}

type wireReaction struct {
	UserID    string `json:"user_id"`
	PostID    string `json:"post_id"`
	EmojiName string `json:"emoji_name"`
	CreateAt  int64  `json:"create_at"`
}

func (r wireReaction) reaction() store.Reaction {
	return store.Reaction{UserID: store.UserID(r.UserID), EmojiName: r.EmojiName, CreateAt: r.CreateAt}
}

type wirePost struct {
	ID            string         `json:"id"`
	CreateAt      int64          `json:"create_at"`
	EditAt        int64          `json:"edit_at"`
	DeleteAt      int64          `json:"delete_at"`
	UserID        string         `json:"user_id"`
	ChannelID     string         `json:"channel_id"`
	RootID        string         `json:"root_id"`
	Message       string         `json:"message"`
	Type          string         `json:"type"`
	Props         map[string]any `json:"props"`
	FileIDs       []string       `json:"file_ids"`
	PendingPostID string         `json:"pending_post_id"`
	ReplyCount    int64          `json:"reply_count"`
	LastReplyAt   int64          `json:"last_reply_at"`
	Participants  []wireUser     `json:"participants"`
	Metadata      *struct {
		Files     []wireFile     `json:"files"`
		Reactions []wireReaction `json:"reactions"`
	} `json:"metadata"`
}

func (p wirePost) post() store.Post {
	out := store.Post{
		ID: store.PostID(p.ID), ChannelID: store.ChannelID(p.ChannelID), UserID: store.UserID(p.UserID),
		RootID: store.PostID(p.RootID), CreateAt: p.CreateAt, EditAt: p.EditAt, Message: p.Message,
		Type: p.Type, PendingID: p.PendingPostID, ReplyCount: p.ReplyCount, LastReplyAt: p.LastReplyAt,
	}
	for _, u := range p.Participants {
		out.Participants = append(out.Participants, store.UserID(u.ID))
	}
	if s, ok := p.Props["override_username"].(string); ok {
		out.Props.OverrideUsername = s
	}
	out.Props.FromBot = p.Props["from_bot"] == "true" || p.Props["from_webhook"] == "true"
	if p.Metadata != nil {
		for _, f := range p.Metadata.Files {
			out.Files = append(out.Files, f.file())
		}
		for _, r := range p.Metadata.Reactions {
			out.Reactions = append(out.Reactions, r.reaction())
		}
	}
	return out
}

type wirePostList struct {
	Order      []string            `json:"order"`
	Posts      map[string]wirePost `json:"posts"`
	PrevPostID string              `json:"prev_post_id"`
	HasNext    *bool               `json:"has_next"`
}

// posts returns the posts of a list in its order, newest first.
func (l wirePostList) posts() []store.Post {
	out := make([]store.Post, 0, len(l.Order))
	for _, id := range l.Order {
		if p, ok := l.Posts[id]; ok && p.DeleteAt == 0 {
			out = append(out, p.post())
		}
	}
	return out
}

type wireStatus struct {
	UserID string `json:"user_id"`
	Status string `json:"status"`
}

type wireError struct {
	ID         string `json:"id"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}
