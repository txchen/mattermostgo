// Package mm talks to a Mattermost server: the REST API v4 and the
// WebSocket. It is the only package that sees the server's JSON, which it
// turns into store values and events.
package mm

import (
	"cmp"
	"encoding/json"
	"fmt"
	"time"

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
	Position          string            `json:"position"`
	MFAActive         bool              `json:"mfa_active"`
	Email             string            `json:"email"`
	Props             map[string]string `json:"props"`
}

// customStatus reads the custom status a user keeps, as JSON, in a prop.
func (u wireUser) customStatus() *store.CustomStatus {
	var w struct {
		Emoji     string    `json:"emoji"`
		Text      string    `json:"text"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if json.Unmarshal([]byte(u.Props["customStatus"]), &w) != nil || w.Emoji == "" && w.Text == "" {
		return nil
	}
	return &store.CustomStatus{Emoji: w.Emoji, Text: w.Text, ExpiresAt: w.ExpiresAt}
}

func (u wireUser) user() store.User {
	return store.User{
		ID: store.UserID(u.ID), Username: u.Username, FirstName: u.FirstName, LastName: u.LastName,
		Nickname: u.Nickname, IsBot: u.IsBot, Deleted: u.DeleteAt != 0, PictureAt: u.LastPictureUpdate,
		DesktopNotify: u.NotifyProps["desktop"], DesktopSound: u.NotifyProps["desktop_sound"], Position: u.Position, Email: u.Email, MFAActive: u.MFAActive, CustomStatus: u.customStatus(),
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
	IsPinned      bool           `json:"is_pinned"`
	Participants  []wireUser     `json:"participants"`
	Metadata      *struct {
		Files     []wireFile               `json:"files"`
		Reactions []wireReaction           `json:"reactions"`
		Embeds    []wireEmbed              `json:"embeds"`
		Images    map[string]wireImageSize `json:"images"`
	} `json:"metadata"`
}

type wireEmbed struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Data *struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
		SiteName    string `json:"site_name"`
		Images      []struct {
			URL       string `json:"url"`
			SecureURL string `json:"secure_url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"images"`
	} `json:"data"`
}

type wireImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// wireAttachment is a message attachment, as webhooks and bots send them
// in a post's props (model.SlackAttachment).
type wireAttachment struct {
	Fallback   string `json:"fallback"`
	Color      string `json:"color"`
	Pretext    string `json:"pretext"`
	AuthorName string `json:"author_name"`
	AuthorLink string `json:"author_link"`
	AuthorIcon string `json:"author_icon"`
	Title      string `json:"title"`
	TitleLink  string `json:"title_link"`
	Text       string `json:"text"`
	Fields     []struct {
		Title string `json:"title"`
		Value any    `json:"value"`
		Short any    `json:"short"`
	} `json:"fields"`
	ImageURL   string `json:"image_url"`
	ThumbURL   string `json:"thumb_url"`
	Footer     string `json:"footer"`
	FooterIcon string `json:"footer_icon"`
}

// embed is the preview the webapp shows for a post's first link: a page's
// OpenGraph card, or the image a link points to.
func (p wirePost) embed() *store.Embed {
	size := func(u string) (int, int) {
		im := p.Metadata.Images[u]
		return im.Width, im.Height
	}
	for _, e := range p.Metadata.Embeds {
		switch e.Type {
		case "image":
			w, h := size(e.URL)
			return &store.Embed{URL: e.URL, Image: &store.Image{URL: e.URL, Width: w, Height: h}, ImageOnly: true}
		case "opengraph":
			if e.Data == nil || e.Data.Title == "" && e.Data.Description == "" {
				continue
			}
			out := &store.Embed{URL: e.URL, Title: e.Data.Title, Description: e.Data.Description, SiteName: e.Data.SiteName}
			for _, im := range e.Data.Images {
				if u := cmp.Or(im.SecureURL, im.URL); u != "" {
					w, h := size(u)
					out.Image = &store.Image{URL: u, Width: cmp.Or(w, im.Width), Height: cmp.Or(h, im.Height)}
					break
				}
			}
			return out
		}
	}
	return nil
}

func (p wirePost) attachments() []store.Attachment {
	raw, ok := p.Props["attachments"]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(raw)
	var ws []wireAttachment
	if json.Unmarshal(b, &ws) != nil {
		return nil
	}
	out := make([]store.Attachment, 0, len(ws))
	for _, w := range ws {
		a := store.Attachment{Fallback: w.Fallback, Color: w.Color, Pretext: w.Pretext, AuthorName: w.AuthorName, AuthorLink: w.AuthorLink,
			AuthorIcon: w.AuthorIcon, Title: w.Title, TitleLink: w.TitleLink, Text: w.Text, ImageURL: w.ImageURL, ThumbURL: w.ThumbURL,
			Footer: w.Footer, FooterIcon: w.FooterIcon}
		for _, f := range w.Fields {
			if f.Title == "" && f.Value == nil {
				continue
			}
			value := ""
			if f.Value != nil {
				value = fmt.Sprint(f.Value)
			}
			a.Fields = append(a.Fields, store.AttachmentField{Title: f.Title, Value: value, Short: f.Short == true || f.Short == "true"})
		}
		out = append(out, a)
	}
	return out
}

func (p wirePost) post() store.Post {
	out := store.Post{
		ID: store.PostID(p.ID), ChannelID: store.ChannelID(p.ChannelID), UserID: store.UserID(p.UserID),
		RootID: store.PostID(p.RootID), CreateAt: p.CreateAt, EditAt: p.EditAt, Message: p.Message,
		Type: p.Type, PendingID: p.PendingPostID, ReplyCount: p.ReplyCount, LastReplyAt: p.LastReplyAt,
		Pinned: p.IsPinned,
	}
	for _, u := range p.Participants {
		out.Participants = append(out.Participants, store.UserID(u.ID))
	}
	if s, ok := p.Props["override_username"].(string); ok {
		out.Props.OverrideUsername = s
	}
	if s, ok := p.Props["override_icon_url"].(string); ok {
		out.Props.OverrideIconURL = s
	}
	out.Props.FromBot = p.Props["from_bot"] == "true" || p.Props["from_webhook"] == "true"
	out.Attachments = p.attachments()
	if p.Metadata != nil {
		if len(out.Attachments) == 0 {
			out.Embed = p.embed()
		}
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
