package mm

import (
	"context"
	"net/url"

	"github.com/txchen/mattermostgo/internal/store"
)

// The integration types are the server's JSON as it is: they go to the
// integrations dialog only, never into a store.

type IncomingHook struct {
	ID            string `json:"id,omitempty"`
	CreateAt      int64  `json:"create_at,omitempty"`
	TeamID        string `json:"team_id,omitempty"`
	ChannelID     string `json:"channel_id"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	Username      string `json:"username,omitempty"`
	IconURL       string `json:"icon_url,omitempty"`
	ChannelLocked bool   `json:"channel_locked"`
}

type OutgoingHook struct {
	ID           string   `json:"id,omitempty"`
	CreateAt     int64    `json:"create_at,omitempty"`
	TeamID       string   `json:"team_id"`
	ChannelID    string   `json:"channel_id,omitempty"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	TriggerWords []string `json:"trigger_words"`
	// TriggerWhen is 0 for a post starting with a trigger word, 1 for
	// one that begins with it.
	TriggerWhen  int      `json:"trigger_when"`
	CallbackURLs []string `json:"callback_urls"`
	ContentType  string   `json:"content_type"`
	Token        string   `json:"token,omitempty"`
}

type Command struct {
	ID               string `json:"id,omitempty"`
	CreateAt         int64  `json:"create_at,omitempty"`
	TeamID           string `json:"team_id"`
	Trigger          string `json:"trigger"`
	Method           string `json:"method"`
	URL              string `json:"url"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	Username         string `json:"username,omitempty"`
	IconURL          string `json:"icon_url,omitempty"`
	AutoComplete     bool   `json:"auto_complete"`
	AutoCompleteDesc string `json:"auto_complete_desc,omitempty"`
	AutoCompleteHint string `json:"auto_complete_hint,omitempty"`
	Token            string `json:"token,omitempty"`
}

type Bot struct {
	UserID      string `json:"user_id,omitempty"`
	CreateAt    int64  `json:"create_at,omitempty"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id,omitempty"`
	DeleteAt    int64  `json:"delete_at,omitempty"`
}

// AccessToken is a personal access token; Token is set only as the
// server creates it.
type AccessToken struct {
	ID          string `json:"id"`
	Token       string `json:"token"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

func teamQuery(team store.TeamID) url.Values {
	return url.Values{"team_id": {string(team)}, "per_page": {"200"}}
}

func (c *Client) IncomingHooks(ctx context.Context, team store.TeamID) ([]IncomingHook, error) {
	var out []IncomingHook
	err := c.do(ctx, "GET", "/hooks/incoming", teamQuery(team), nil, &out)
	return out, err
}

func (c *Client) CreateIncomingHook(ctx context.Context, h IncomingHook) (IncomingHook, error) {
	var out IncomingHook
	err := c.do(ctx, "POST", "/hooks/incoming", nil, h, &out)
	return out, err
}

func (c *Client) DeleteIncomingHook(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/hooks/incoming/"+id, nil, nil, nil)
}

// HookURL is the address an incoming webhook takes posts at.
func (c *Client) HookURL(id string) string {
	u := *c.base
	u.Path += "/hooks/" + id
	return u.String()
}

func (c *Client) OutgoingHooks(ctx context.Context, team store.TeamID) ([]OutgoingHook, error) {
	var out []OutgoingHook
	err := c.do(ctx, "GET", "/hooks/outgoing", teamQuery(team), nil, &out)
	return out, err
}

func (c *Client) CreateOutgoingHook(ctx context.Context, h OutgoingHook) (OutgoingHook, error) {
	var out OutgoingHook
	err := c.do(ctx, "POST", "/hooks/outgoing", nil, h, &out)
	return out, err
}

func (c *Client) DeleteOutgoingHook(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/hooks/outgoing/"+id, nil, nil, nil)
}

func (c *Client) Commands(ctx context.Context, team store.TeamID) ([]Command, error) {
	var out []Command
	err := c.do(ctx, "GET", "/commands", url.Values{"team_id": {string(team)}, "custom_only": {"true"}}, nil, &out)
	return out, err
}

func (c *Client) CreateCommand(ctx context.Context, cmd Command) (Command, error) {
	var out Command
	err := c.do(ctx, "POST", "/commands", nil, cmd, &out)
	return out, err
}

func (c *Client) DeleteCommand(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/commands/"+id, nil, nil, nil)
}

func (c *Client) Bots(ctx context.Context) ([]Bot, error) {
	var out []Bot
	err := c.do(ctx, "GET", "/bots", url.Values{"include_deleted": {"true"}, "per_page": {"200"}}, nil, &out)
	return out, err
}

func (c *Client) CreateBot(ctx context.Context, b Bot) (Bot, error) {
	var out Bot
	err := c.do(ctx, "POST", "/bots", nil, b, &out)
	return out, err
}

// SetBotEnabled enables a bot, or disables it, which keeps its posts and
// stops its tokens working.
func (c *Client) SetBotEnabled(ctx context.Context, id string, enabled bool) (Bot, error) {
	action := "/disable"
	if enabled {
		action = "/enable"
	}
	var out Bot
	err := c.do(ctx, "POST", "/bots/"+id+action, nil, nil, &out)
	return out, err
}

func (c *Client) CreateToken(ctx context.Context, user, description string) (AccessToken, error) {
	var out AccessToken
	err := c.do(ctx, "POST", "/users/"+user+"/tokens", nil, map[string]string{"description": description}, &out)
	return out, err
}

func (c *Client) Tokens(ctx context.Context, user string) ([]AccessToken, error) {
	var out []AccessToken
	err := c.do(ctx, "GET", "/users/"+user+"/tokens", url.Values{"per_page": {"200"}}, nil, &out)
	return out, err
}

func (c *Client) RevokeToken(ctx context.Context, id string) error {
	return c.do(ctx, "POST", "/users/tokens/revoke", nil, map[string]string{"token_id": id}, nil)
}

// AddToTeam makes a user, as a new bot, a member of a team.
func (c *Client) AddToTeam(ctx context.Context, team store.TeamID, user string) error {
	return c.do(ctx, "POST", "/teams/"+string(team)+"/members", nil, map[string]string{"team_id": string(team), "user_id": user}, nil)
}

func (c *Client) UpdateIncomingHook(ctx context.Context, h IncomingHook) (IncomingHook, error) {
	var out IncomingHook
	err := c.do(ctx, "PUT", "/hooks/incoming/"+h.ID, nil, h, &out)
	return out, err
}

func (c *Client) UpdateOutgoingHook(ctx context.Context, h OutgoingHook) (OutgoingHook, error) {
	var out OutgoingHook
	err := c.do(ctx, "PUT", "/hooks/outgoing/"+h.ID, nil, h, &out)
	return out, err
}

func (c *Client) UpdateCommand(ctx context.Context, cmd Command) (Command, error) {
	var out Command
	err := c.do(ctx, "PUT", "/commands/"+cmd.ID, nil, cmd, &out)
	return out, err
}

// UpdateBot changes the names and the description of a bot.
func (c *Client) UpdateBot(ctx context.Context, b Bot) (Bot, error) {
	var out Bot
	body := map[string]string{"username": b.Username, "display_name": b.DisplayName, "description": b.Description}
	err := c.do(ctx, "PUT", "/bots/"+b.UserID, nil, body, &out)
	return out, err
}
