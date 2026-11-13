package mm

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/txchen/mattermostgo/internal/store"
)

// ChannelPatch holds the channel fields to change; nil leaves one as it is.
type ChannelPatch struct {
	DisplayName *string `json:"display_name,omitempty"`
	Name        *string `json:"name,omitempty"`
	Header      *string `json:"header,omitempty"`
	Purpose     *string `json:"purpose,omitempty"`
}

func (c *Client) PatchChannel(ctx context.Context, id store.ChannelID, p ChannelPatch) (store.Channel, error) {
	var w wireChannel
	err := c.do(ctx, "PUT", "/channels/"+string(id)+"/patch", nil, p, &w)
	return w.channel(), err
}

// MakePrivate converts a public channel to a private one.
func (c *Client) MakePrivate(ctx context.Context, id store.ChannelID) (store.Channel, error) {
	var w wireChannel
	err := c.do(ctx, "PUT", "/channels/"+string(id)+"/privacy", nil, map[string]string{"privacy": "P"}, &w)
	return w.channel(), err
}

// ArchiveChannel archives a channel, which the API calls deleting it.
func (c *Client) ArchiveChannel(ctx context.Context, id store.ChannelID) error {
	return c.do(ctx, "DELETE", "/channels/"+string(id), nil, nil, nil)
}

// NewChannel is a channel to create in a team.
type NewChannel struct {
	TeamID      store.TeamID      `json:"team_id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Type        store.ChannelType `json:"type"`
	Purpose     string            `json:"purpose,omitempty"`
}

func (c *Client) CreateChannel(ctx context.Context, n NewChannel) (store.Channel, error) {
	var w wireChannel
	err := c.do(ctx, "POST", "/channels", nil, n, &w)
	return w.channel(), err
}

// PublicChannels lists a team's public channels, joined or not.
func (c *Client) PublicChannels(ctx context.Context, team store.TeamID) ([]store.Channel, error) {
	var ws []wireChannel
	if err := c.do(ctx, "GET", "/teams/"+string(team)+"/channels", url.Values{"per_page": {"200"}}, nil, &ws); err != nil {
		return nil, err
	}
	out := make([]store.Channel, 0, len(ws))
	for _, w := range ws {
		if w.DeleteAt == 0 {
			out = append(out, w.channel())
		}
	}
	return out, nil
}

func (c *Client) AddMembers(ctx context.Context, id store.ChannelID, users []store.UserID) error {
	return c.do(ctx, "POST", "/channels/"+string(id)+"/members", nil, map[string]any{"user_ids": users}, nil)
}

// RemoveMember takes a user out of a channel; the current user leaves it.
func (c *Client) RemoveMember(ctx context.Context, id store.ChannelID, user store.UserID) error {
	return c.do(ctx, "DELETE", "/channels/"+string(id)+"/members/"+string(user), nil, nil, nil)
}

// MemberInfo is a member of a channel, as its members list shows them.
type MemberInfo struct {
	User  store.UserID
	Admin bool
}

func (c *Client) Members(ctx context.Context, id store.ChannelID) ([]MemberInfo, error) {
	var ws []struct {
		UserID      string `json:"user_id"`
		Roles       string `json:"roles"`
		SchemeAdmin bool   `json:"scheme_admin"`
	}
	if err := c.do(ctx, "GET", "/channels/"+string(id)+"/members", url.Values{"per_page": {"200"}}, nil, &ws); err != nil {
		return nil, err
	}
	out := make([]MemberInfo, len(ws))
	for i, w := range ws {
		out[i] = MemberInfo{User: store.UserID(w.UserID), Admin: w.SchemeAdmin || slices.Contains(strings.Fields(w.Roles), "channel_admin")}
	}
	return out, nil
}

// SetMarkUnread sets when a channel counts as unread: "all", or
// "mention" for a muted channel.
func (c *Client) SetMarkUnread(ctx context.Context, me store.UserID, id store.ChannelID, markUnread string) error {
	return c.do(ctx, "PUT", "/channels/"+string(id)+"/members/"+string(me)+"/notify_props", nil, map[string]string{"channel_id": string(id), "user_id": string(me), "mark_unread": markUnread}, nil)
}

// MarkUnread marks a post and those after it unread, as the webapp's
// Mark as Unread does with a channel's last post.
func (c *Client) MarkUnread(ctx context.Context, me store.UserID, post store.PostID) error {
	return c.do(ctx, "POST", "/users/"+string(me)+"/posts/"+string(post)+"/set_unread", nil, map[string]bool{"collapsed_threads_supported": true}, nil)
}

// LastPost returns the newest root post of a channel, or "" for none.
func (c *Client) LastPost(ctx context.Context, id store.ChannelID) (store.PostID, error) {
	var l wirePostList
	if err := c.do(ctx, "GET", "/channels/"+string(id)+"/posts", url.Values{"per_page": {"1"}, "collapsedThreads": {"true"}}, nil, &l); err != nil {
		return "", err
	}
	if len(l.Order) == 0 {
		return "", nil
	}
	return store.PostID(l.Order[0]), nil
}

func (c *Client) categoriesPath(team store.TeamID) string {
	return "/users/me/teams/" + string(team) + "/channels/categories"
}

// MoveToCategory files a channel under a category of the sidebar, at its
// top, and takes it out of the others. The server leaves a channel in
// every category that lists it, so the client updates them together, as
// the webapp does.
func (c *Client) MoveToCategory(ctx context.Context, team store.TeamID, id store.ChannelID, category string) error {
	// Categories go back whole, with the fields the client does not
	// model, so they round-trip as maps.
	var all struct {
		Categories []map[string]any `json:"categories"`
	}
	if err := c.do(ctx, "GET", c.categoriesPath(team), nil, nil, &all); err != nil {
		return err
	}
	var changed []map[string]any
	for _, cat := range all.Categories {
		old, _ := cat["channel_ids"].([]any)
		ids := []any{}
		if cat["id"] == category {
			ids = append(ids, string(id))
		}
		for _, x := range old {
			if x != string(id) {
				ids = append(ids, x)
			}
		}
		if cat["id"] == category || len(ids) != len(old) {
			cat["channel_ids"] = ids
			changed = append(changed, cat)
		}
	}
	return c.do(ctx, "PUT", c.categoriesPath(team), nil, changed, nil)
}

// CreateCategory makes an empty custom category and returns its ID.
func (c *Client) CreateCategory(ctx context.Context, me store.UserID, team store.TeamID, name string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	body := map[string]any{"user_id": me, "team_id": team, "display_name": name, "type": "custom", "channel_ids": []string{}}
	err := c.do(ctx, "POST", c.categoriesPath(team), nil, body, &out)
	return out.ID, err
}

func (c *Client) RenameCategory(ctx context.Context, team store.TeamID, category, name string) error {
	var cat map[string]any
	if err := c.do(ctx, "GET", c.categoriesPath(team)+"/"+category, nil, nil, &cat); err != nil {
		return err
	}
	cat["display_name"] = name
	return c.do(ctx, "PUT", c.categoriesPath(team)+"/"+category, nil, cat, nil)
}

// DeleteCategory deletes a custom category; its channels go back to
// their default categories.
func (c *Client) DeleteCategory(ctx context.Context, team store.TeamID, category string) error {
	return c.do(ctx, "DELETE", c.categoriesPath(team)+"/"+category, nil, nil, nil)
}

// SetStatus sets the status of the user; DND with dndEnd set ends then.
func (c *Client) SetStatus(ctx context.Context, me store.UserID, status store.Status, dndEnd time.Time) error {
	body := map[string]any{"user_id": me, "status": status}
	if status == store.DND && !dndEnd.IsZero() {
		body["dnd_end_time"] = dndEnd.Unix()
	}
	return c.do(ctx, "PUT", "/users/"+string(me)+"/status", nil, body, nil)
}

// SetCustomStatus sets the emoji and text by the name of the user.
// duration names when it clears, as the webapp does, empty for never.
func (c *Client) SetCustomStatus(ctx context.Context, cs store.CustomStatus, duration string) error {
	body := map[string]any{"emoji": cs.Emoji, "text": cs.Text, "duration": duration}
	if !cs.ExpiresAt.IsZero() {
		body["expires_at"] = cs.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return c.do(ctx, "PUT", "/users/me/status/custom", nil, body, nil)
}

func (c *Client) ClearCustomStatus(ctx context.Context) error {
	return c.do(ctx, "DELETE", "/users/me/status/custom", nil, nil, nil)
}

// UserPatch holds the profile fields to change.
type UserPatch struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Nickname  string `json:"nickname"`
	Position  string `json:"position"`
}

func (c *Client) PatchMe(ctx context.Context, p UserPatch) (store.User, error) {
	var w wireUser
	err := c.do(ctx, "PUT", "/users/me/patch", nil, p, &w)
	return w.user(), err
}
