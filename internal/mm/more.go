package mm

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/url"
	"strconv"
	"time"

	"github.com/txchen/mattermostgo/internal/store"
)

type wirePref struct {
	UserID   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

func (p wirePref) pref() store.Preference {
	return store.Preference{Category: p.Category, Name: p.Name, Value: p.Value}
}

func (c *Client) Preferences(ctx context.Context) ([]store.Preference, error) {
	var ws []wirePref
	if err := c.do(ctx, "GET", "/users/me/preferences", nil, nil, &ws); err != nil {
		return nil, err
	}
	out := make([]store.Preference, len(ws))
	for i, w := range ws {
		out[i] = w.pref()
	}
	return out, nil
}

func (c *Client) SetPrefs(ctx context.Context, me store.UserID, prefs []store.Preference) error {
	ws := make([]wirePref, len(prefs))
	for i, p := range prefs {
		ws[i] = wirePref{UserID: string(me), Category: p.Category, Name: p.Name, Value: p.Value}
	}
	return c.do(ctx, "PUT", "/users/"+string(me)+"/preferences", nil, ws, nil)
}

func (c *Client) DeletePrefs(ctx context.Context, me store.UserID, prefs []store.Preference) error {
	ws := make([]wirePref, len(prefs))
	for i, p := range prefs {
		ws[i] = wirePref{UserID: string(me), Category: p.Category, Name: p.Name, Value: p.Value}
	}
	return c.do(ctx, "POST", "/users/"+string(me)+"/preferences/delete", nil, ws, nil)
}

// PostResults are posts the server found or lists, newest first.
type PostResults struct {
	Posts []store.Post
}

func (l wirePostList) results() PostResults {
	return PostResults{Posts: l.posts()}
}

// Search finds posts in a team, with the webapp's terms (from:, in:,
// before:, on:, after:, "phrases").
func (c *Client) Search(ctx context.Context, team store.TeamID, terms string) (PostResults, error) {
	var l wirePostList
	_, offset := time.Now().Zone()
	body := map[string]any{"terms": terms, "is_or_search": false, "time_zone_offset": offset, "page": 0, "per_page": 60, "include_deleted_channels": false}
	err := c.do(ctx, "POST", "/teams/"+string(team)+"/posts/search", nil, body, &l)
	return l.results(), err
}

// SearchFiles finds files in a team.
func (c *Client) SearchFiles(ctx context.Context, team store.TeamID, terms string) ([]FileResult, error) {
	var out struct {
		Order     []string `json:"order"`
		FileInfos map[string]struct {
			wireFile
			ChannelID string `json:"channel_id"`
			PostID    string `json:"post_id"`
			UserID    string `json:"user_id"`
			CreateAt  int64  `json:"create_at"`
		} `json:"file_infos"`
	}
	body := map[string]any{"terms": terms, "is_or_search": false, "page": 0, "per_page": 60}
	if err := c.do(ctx, "POST", "/teams/"+string(team)+"/files/search", nil, body, &out); err != nil {
		return nil, err
	}
	var res []FileResult
	for _, id := range out.Order {
		if f, ok := out.FileInfos[id]; ok {
			res = append(res, FileResult{File: f.file(), Channel: store.ChannelID(f.ChannelID), Post: store.PostID(f.PostID), User: store.UserID(f.UserID), CreateAt: f.CreateAt})
		}
	}
	return res, nil
}

// FileResult is a file found, with where it was posted.
type FileResult struct {
	File     store.FileInfo
	Channel  store.ChannelID
	Post     store.PostID
	User     store.UserID
	CreateAt int64
}

// Saved lists the posts the user saved.
func (c *Client) Saved(ctx context.Context, me store.UserID) (PostResults, error) {
	var l wirePostList
	err := c.do(ctx, "GET", "/users/"+string(me)+"/posts/flagged", url.Values{"per_page": {"60"}}, nil, &l)
	return l.results(), err
}

func (c *Client) Pinned(ctx context.Context, ch store.ChannelID) (PostResults, error) {
	var l wirePostList
	err := c.do(ctx, "GET", "/channels/"+string(ch)+"/pinned", nil, nil, &l)
	return l.results(), err
}

func (c *Client) SetPinned(ctx context.Context, id store.PostID, pinned bool) error {
	action := "/unpin"
	if pinned {
		action = "/pin"
	}
	return c.do(ctx, "POST", "/posts/"+string(id)+action, nil, nil, nil)
}

// Remind asks the server to remind the user of a post at a time.
func (c *Client) Remind(ctx context.Context, me store.UserID, id store.PostID, at time.Time) error {
	return c.do(ctx, "POST", "/users/"+string(me)+"/posts/"+string(id)+"/reminder", nil, map[string]int64{"target_time": at.Unix()}, nil)
}

// EditHistory returns the earlier versions of a post, newest first.
func (c *Client) EditHistory(ctx context.Context, id store.PostID) ([]store.Post, error) {
	var ws []wirePost
	if err := c.do(ctx, "GET", "/posts/"+string(id)+"/edit_history", nil, nil, &ws); err != nil {
		return nil, err
	}
	out := make([]store.Post, len(ws))
	for i, w := range ws {
		out[i] = w.post()
	}
	return out, nil
}

type wireThread struct {
	ID             string     `json:"id"`
	ReplyCount     int64      `json:"reply_count"`
	LastReplyAt    int64      `json:"last_reply_at"`
	LastViewedAt   int64      `json:"last_viewed_at"`
	UnreadReplies  int64      `json:"unread_replies"`
	UnreadMentions int64      `json:"unread_mentions"`
	IsFollowing    *bool      `json:"is_following"`
	Participants   []wireUser `json:"participants"`
	Post           wirePost   `json:"post"`
}

func (w wireThread) state(team store.TeamID) store.ThreadState {
	t := store.ThreadState{Root: store.PostID(w.ID), Team: team, ReplyCount: w.ReplyCount, LastReplyAt: w.LastReplyAt,
		LastViewedAt: w.LastViewedAt, UnreadReplies: w.UnreadReplies, UnreadMentions: w.UnreadMentions, Following: w.IsFollowing == nil || *w.IsFollowing}
	for _, u := range w.Participants {
		t.Participants = append(t.Participants, store.UserID(u.ID))
	}
	return t
}

func (c *Client) threadsPath(me store.UserID, team store.TeamID) string {
	return "/users/" + string(me) + "/teams/" + string(team) + "/threads"
}

// Threads lists the threads the user follows in a team, newest first.
func (c *Client) Threads(ctx context.Context, me store.UserID, team store.TeamID) (store.ThreadsLoaded, error) {
	var out struct {
		Threads []wireThread `json:"threads"`
	}
	q := url.Values{"per_page": {"50"}, "extended": {"true"}}
	if err := c.do(ctx, "GET", c.threadsPath(me, team), q, nil, &out); err != nil {
		return store.ThreadsLoaded{}, err
	}
	ev := store.ThreadsLoaded{Team: team}
	for _, w := range out.Threads {
		ev.Threads = append(ev.Threads, w.state(team))
		p := w.Post.post()
		p.ReplyCount, p.LastReplyAt = w.ReplyCount, w.LastReplyAt
		for _, u := range w.Participants {
			p.Participants = append(p.Participants, store.UserID(u.ID))
		}
		ev.Posts = append(ev.Posts, p)
	}
	return ev, nil
}

func (c *Client) SetFollowing(ctx context.Context, me store.UserID, team store.TeamID, root store.PostID, on bool) error {
	method := "DELETE"
	if on {
		method = "PUT"
	}
	return c.do(ctx, method, c.threadsPath(me, team)+"/"+string(root)+"/following", nil, nil, nil)
}

// ReadThread marks a thread read up to a time in milliseconds.
func (c *Client) ReadThread(ctx context.Context, me store.UserID, team store.TeamID, root store.PostID, at int64) error {
	return c.do(ctx, "PUT", c.threadsPath(me, team)+"/"+string(root)+"/read/"+strconv.FormatInt(at, 10), nil, nil, nil)
}

// AutocompleteCommand is a slash command the composer suggests.
type AutocompleteCommand struct {
	Trigger     string `json:"trigger"`
	Hint        string `json:"auto_complete_hint"`
	Description string `json:"auto_complete_desc"`
	DisplayName string `json:"display_name"`
}

// Commands lists the slash commands of a team the user may run, built-in
// ones included.
func (c *Client) AutocompleteCommands(ctx context.Context, team store.TeamID) ([]AutocompleteCommand, error) {
	var out []AutocompleteCommand
	err := c.do(ctx, "GET", "/teams/"+string(team)+"/commands/autocomplete", nil, nil, &out)
	return out, err
}

// CommandResult is what running a slash command answers: a URL to open,
// or text only the user sees.
type CommandResult struct {
	GotoLocation string `json:"goto_location"`
	Text         string `json:"text"`
	ResponseType string `json:"response_type"`
}

// RunCommand runs a slash command in a channel, as the webapp does.
func (c *Client) RunCommand(ctx context.Context, team store.TeamID, ch store.ChannelID, root store.PostID, command string) (CommandResult, error) {
	var out CommandResult
	body := map[string]string{"command": command, "channel_id": string(ch), "team_id": string(team), "root_id": string(root)}
	err := c.do(ctx, "POST", "/commands/execute", nil, body, &out)
	return out, err
}

// CustomEmoji is an emoji the server's users uploaded.
type CustomEmoji struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) CustomEmojis(ctx context.Context) ([]CustomEmoji, error) {
	var all []CustomEmoji
	for page := 0; page < 20; page++ {
		var out []CustomEmoji
		if err := c.do(ctx, "GET", "/emoji", url.Values{"page": {strconv.Itoa(page)}, "per_page": {"200"}, "sort": {"name"}}, nil, &out); err != nil {
			return all, err
		}
		all = append(all, out...)
		if len(out) < 200 {
			break
		}
	}
	return all, nil
}

func (c *Client) EmojiImage(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	err := c.do(ctx, "GET", "/emoji/"+id+"/image", nil, nil, &b)
	return b, err
}

// GroupChannel returns the group message of the users, the current one
// among them, creating it if needed.
func (c *Client) GroupChannel(ctx context.Context, users []store.UserID) (store.Channel, error) {
	var ch wireChannel
	err := c.do(ctx, "POST", "/channels/group", nil, users, &ch)
	return ch.channel(), err
}

func (c *Client) SetChannelAdmin(ctx context.Context, id store.ChannelID, user store.UserID, admin bool) error {
	return c.do(ctx, "PUT", "/channels/"+string(id)+"/members/"+string(user)+"/schemeRoles", nil, map[string]bool{"scheme_admin": admin, "scheme_user": true}, nil)
}

func (c *Client) RestoreChannel(ctx context.Context, id store.ChannelID) (store.Channel, error) {
	var w wireChannel
	err := c.do(ctx, "POST", "/channels/"+string(id)+"/restore", nil, nil, &w)
	return w.channel(), err
}

func (c *Client) MakePublic(ctx context.Context, id store.ChannelID) (store.Channel, error) {
	var w wireChannel
	err := c.do(ctx, "PUT", "/channels/"+string(id)+"/privacy", nil, map[string]string{"privacy": "O"}, &w)
	return w.channel(), err
}

// ArchivedChannels lists the archived channels of a team the user may see.
func (c *Client) ArchivedChannels(ctx context.Context, team store.TeamID) ([]store.Channel, error) {
	var ws []wireChannel
	if err := c.do(ctx, "GET", "/teams/"+string(team)+"/channels/deleted", url.Values{"per_page": {"200"}}, nil, &ws); err != nil {
		return nil, err
	}
	out := make([]store.Channel, len(ws))
	for i, w := range ws {
		out[i] = w.channel()
	}
	return out, nil
}

// Bookmark is a link or file pinned to the top of a channel.
type Bookmark struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"display_name"`
	LinkURL     string `json:"link_url,omitempty"`
	Emoji       string `json:"emoji,omitempty"`
	Type        string `json:"type"`
	FileID      string `json:"file_id,omitempty"`
}

func (c *Client) Bookmarks(ctx context.Context, ch store.ChannelID) ([]Bookmark, error) {
	var out []Bookmark
	err := c.do(ctx, "GET", "/channels/"+string(ch)+"/bookmarks", nil, nil, &out)
	return out, err
}

func (c *Client) AddBookmark(ctx context.Context, ch store.ChannelID, b Bookmark) (Bookmark, error) {
	var out Bookmark
	b.Type = "link"
	err := c.do(ctx, "POST", "/channels/"+string(ch)+"/bookmarks", nil, b, &out)
	return out, err
}

func (c *Client) DeleteBookmark(ctx context.Context, ch store.ChannelID, id string) error {
	return c.do(ctx, "DELETE", "/channels/"+string(ch)+"/bookmarks/"+id, nil, nil, nil)
}

// SetNotify changes the notification settings of the user, merged with
// the others the server keeps.
func (c *Client) SetNotify(ctx context.Context, props map[string]string) (store.User, error) {
	var me wireUser
	if err := c.do(ctx, "GET", "/users/me", nil, nil, &me); err != nil {
		return store.User{}, err
	}
	merged := map[string]string{}
	for k, v := range me.NotifyProps {
		merged[k] = v
	}
	for k, v := range props {
		merged[k] = v
	}
	var w wireUser
	err := c.do(ctx, "PUT", "/users/me/patch", nil, map[string]any{"notify_props": merged}, &w)
	return w.user(), err
}

// SetChannelNotify changes the notification settings of the user in a
// channel.
func (c *Client) SetChannelNotify(ctx context.Context, me store.UserID, id store.ChannelID, props map[string]string) error {
	body := map[string]string{"channel_id": string(id), "user_id": string(me)}
	for k, v := range props {
		body[k] = v
	}
	return c.do(ctx, "PUT", "/channels/"+string(id)+"/members/"+string(me)+"/notify_props", nil, body, nil)
}

// NotifyProps returns the notification settings of the user as the
// server keeps them.
func (c *Client) NotifyProps(ctx context.Context) (map[string]string, error) {
	var me wireUser
	err := c.do(ctx, "GET", "/users/me", nil, nil, &me)
	return me.NotifyProps, err
}

// ChannelNotifyProps returns the notification settings of the user in a
// channel.
func (c *Client) ChannelNotifyProps(ctx context.Context, id store.ChannelID) (map[string]string, error) {
	var m wireMember
	err := c.do(ctx, "GET", "/channels/"+string(id)+"/members/me", nil, nil, &m)
	return m.NotifyProps, err
}

func (c *Client) SetProfileImage(ctx context.Context, me store.UserID, name string, data []byte) error {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, err := w.CreateFormFile("image", name)
	if err != nil {
		return err
	}
	part.Write(data)
	w.Close()
	return c.do(ctx, "POST", "/users/"+string(me)+"/image", nil, &multipartBody{buf, w.FormDataContentType()}, nil)
}

func (c *Client) ChangePassword(ctx context.Context, me store.UserID, current, next string) error {
	return c.do(ctx, "PUT", "/users/"+string(me)+"/password", nil, map[string]string{"current_password": current, "new_password": next}, nil)
}

// PatchAccount changes the username or the email, which needs the
// password.
func (c *Client) PatchAccount(ctx context.Context, username, email, password string) (store.User, error) {
	body := map[string]string{"username": username}
	if email != "" {
		body["email"], body["password"] = email, password
	}
	var w wireUser
	err := c.do(ctx, "PUT", "/users/me/patch", nil, body, &w)
	return w.user(), err
}

// MFASecret is a new MFA secret to enter in an authenticator app.
type MFASecret struct {
	Secret string `json:"secret"`
	QRCode []byte `json:"qr_code"`
}

func (c *Client) GenerateMFA(ctx context.Context, me store.UserID) (MFASecret, error) {
	var raw struct {
		Secret string `json:"secret"`
		QRCode string `json:"qr_code"`
	}
	if err := c.do(ctx, "POST", "/users/"+string(me)+"/mfa/generate", nil, nil, &raw); err != nil {
		return MFASecret{}, err
	}
	var qr []byte
	_ = json.Unmarshal([]byte(`"`+raw.QRCode+`"`), &qr)
	return MFASecret{Secret: raw.Secret, QRCode: qr}, nil
}

// SetMFA turns MFA on with a code from the authenticator, or off.
func (c *Client) SetMFA(ctx context.Context, me store.UserID, on bool, code string) error {
	return c.do(ctx, "PUT", "/users/"+string(me)+"/mfa", nil, map[string]any{"activate": on, "code": code}, nil)
}

// Session is a login of the user on a device.
type Session struct {
	ID             string `json:"id"`
	CreateAt       int64  `json:"create_at"`
	LastActivityAt int64  `json:"last_activity_at"`
	DeviceID       string `json:"device_id"`
	Props          struct {
		Platform string `json:"platform"`
		OS       string `json:"os"`
		Browser  string `json:"browser"`
		Type     string `json:"type"`
	} `json:"props"`
}

func (c *Client) Sessions(ctx context.Context, me store.UserID) ([]Session, error) {
	var out []Session
	err := c.do(ctx, "GET", "/users/"+string(me)+"/sessions", nil, nil, &out)
	return out, err
}

func (c *Client) RevokeSession(ctx context.Context, me store.UserID, id string) error {
	return c.do(ctx, "POST", "/users/"+string(me)+"/sessions/revoke", nil, map[string]string{"session_id": id}, nil)
}

// Team returns a team with its invite ID, for the invite link.
func (c *Client) TeamInvite(ctx context.Context, team store.TeamID) (string, error) {
	var t struct {
		InviteID string `json:"invite_id"`
	}
	err := c.do(ctx, "GET", "/teams/"+string(team), nil, nil, &t)
	return t.InviteID, err
}

// InviteLink is where someone joins a team with its invite ID.
func (c *Client) InviteLink(inviteID string) string {
	u := *c.base
	u.Path += "/signup_user_complete/"
	u.RawQuery = url.Values{"id": {inviteID}}.Encode()
	return u.String()
}

func (c *Client) InviteEmails(ctx context.Context, team store.TeamID, emails []string) error {
	return c.do(ctx, "POST", "/teams/"+string(team)+"/invite/email", nil, emails, nil)
}

// AddTeamMembers adds users of the server to a team.
func (c *Client) AddTeamMembers(ctx context.Context, team store.TeamID, users []store.UserID) error {
	body := make([]map[string]string, len(users))
	for i, u := range users {
		body[i] = map[string]string{"team_id": string(team), "user_id": string(u)}
	}
	return c.do(ctx, "POST", "/teams/"+string(team)+"/members/batch", nil, body, nil)
}

// PatchCategory changes fields of a sidebar category, as its sorting,
// muted or collapsed.
func (c *Client) PatchCategory(ctx context.Context, team store.TeamID, category string, fields map[string]any) error {
	var cat map[string]any
	if err := c.do(ctx, "GET", c.categoriesPath(team)+"/"+category, nil, nil, &cat); err != nil {
		return err
	}
	for k, v := range fields {
		cat[k] = v
	}
	return c.do(ctx, "PUT", c.categoriesPath(team)+"/"+category, nil, cat, nil)
}

// SetCategoryChannels puts the channels of a category in an order, the
// manual sorting.
func (c *Client) SetCategoryChannels(ctx context.Context, team store.TeamID, category string, channels []store.ChannelID) error {
	return c.PatchCategory(ctx, team, category, map[string]any{"channel_ids": channels, "sorting": "manual"})
}

func (c *Client) SetCategoryOrder(ctx context.Context, team store.TeamID, order []string) error {
	return c.do(ctx, "PUT", c.categoriesPath(team)+"/order", nil, order, nil)
}

// DoAction chooses a button or a menu option of a post, which the server
// passes to its integration.
func (c *Client) DoAction(ctx context.Context, post store.PostID, action, selected string) error {
	body := map[string]string{}
	if selected != "" {
		body["selected_option"] = selected
	}
	return c.do(ctx, "POST", "/posts/"+string(post)+"/actions/"+action, nil, body, nil)
}

// SubmitDialog sends an interactive dialog to its integration, which may
// answer with errors by field name.
func (c *Client) SubmitDialog(ctx context.Context, d store.Dialog, me store.UserID, ch store.ChannelID, team store.TeamID, values map[string]any, cancelled bool) (map[string]string, error) {
	body := map[string]any{"url": d.URL, "callback_id": d.CallbackID, "state": d.State, "user_id": me, "channel_id": ch,
		"team_id": team, "submission": values, "cancelled": cancelled}
	var out struct {
		Error  string            `json:"error"`
		Errors map[string]string `json:"errors"`
	}
	if err := c.do(ctx, "POST", "/actions/dialogs/submit", nil, body, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		if out.Errors == nil {
			out.Errors = map[string]string{}
		}
		out.Errors[""] = out.Error
	}
	return out.Errors, nil
}
