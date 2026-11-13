package mm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/txchen/mattermostgo/internal/store"
)

// PageSize is how many posts a page of a channel holds.
const PageSize = 60

// APIError is an error the server answered with.
type APIError struct {
	Status  int
	ID      string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("server answered %d %s", e.Status, e.ID)
}

var (
	// ErrMFARequired means the account needs an MFA code to log in.
	ErrMFARequired = errors.New("MFA code required")
	// ErrUnauthorized means the session token is no longer valid.
	ErrUnauthorized = errors.New("session expired")
)

// Client makes requests to one server with one session token.
type Client struct {
	base  *url.URL
	token string
	hc    *http.Client
}

func New(base *url.URL, token string, hc *http.Client) *Client {
	return &Client{base: base, token: token, hc: hc}
}

func (c *Client) Token() string { return c.token }

// ParseServerURL reads a URL the user typed, adding https:// when it has
// no scheme.
func ParseServerURL(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("not a server address: %s", s)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery, u.Fragment = "", ""
	return u, nil
}

func (c *Client) url(path string, q url.Values) string {
	u := *c.base
	u.Path += "/api/v4" + path
	if q != nil {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	var r io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case *multipartBody:
		r, contentType = b.buf, b.contentType
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			return err
		}
		r, contentType = bytes.NewReader(buf), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path, q), r)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return readError(res)
	}
	switch out := out.(type) {
	case nil:
		return nil
	case *[]byte:
		*out, err = io.ReadAll(res.Body)
		return err
	case *http.Header:
		*out = res.Header
		return nil
	case func(*json.Decoder) error:
		return out(json.NewDecoder(res.Body))
	default:
		return json.NewDecoder(res.Body).Decode(out)
	}
}

func readError(res *http.Response) error {
	var we wireError
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&we)
	if res.StatusCode == http.StatusUnauthorized && !strings.HasPrefix(we.ID, "api.user.check_user_mfa") && !strings.HasPrefix(we.ID, "mfa.") && !strings.HasPrefix(we.ID, "api.user.login") {
		return ErrUnauthorized
	}
	return &APIError{Status: res.StatusCode, ID: we.ID, Message: we.Message}
}

// Ping checks that a URL is a Mattermost server.
func Ping(ctx context.Context, hc *http.Client, base *url.URL) error {
	var out struct {
		Status string `json:"status"`
	}
	if err := New(base, "", hc).do(ctx, "GET", "/system/ping", nil, nil, &out); err != nil {
		return err
	}
	if out.Status != "OK" {
		return fmt.Errorf("not a Mattermost server")
	}
	return nil
}

// Login logs in with a password and an MFA code, which may be empty. It
// returns a client with the session's token.
func Login(ctx context.Context, hc *http.Client, base *url.URL, loginID, password, mfa string) (*Client, store.User, error) {
	c := New(base, "", hc)
	body := map[string]string{"login_id": loginID, "password": password}
	if mfa != "" {
		body["token"] = mfa
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.url("/users/login", nil), jsonBody(body))
	if err != nil {
		return nil, store.User{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	res, err := hc.Do(req)
	if err != nil {
		return nil, store.User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		err := readError(res)
		var ae *APIError
		if errors.As(err, &ae) && ae.ID == "mfa.validate_token.authenticate.app_error" && mfa == "" {
			return nil, store.User{}, ErrMFARequired
		}
		return nil, store.User{}, err
	}
	var u wireUser
	if err := json.NewDecoder(res.Body).Decode(&u); err != nil {
		return nil, store.User{}, err
	}
	c.token = res.Header.Get("Token")
	if c.token == "" {
		return nil, store.User{}, errors.New("the server sent no session token")
	}
	return c, u.user(), nil
}

func jsonBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, "POST", "/users/logout", nil, nil, nil)
}

func (c *Client) Me(ctx context.Context) (store.User, error) {
	var u wireUser
	err := c.do(ctx, "GET", "/users/me", nil, nil, &u)
	return u.user(), err
}

// Load fetches what a store.Loaded holds.
func (c *Client) Load(ctx context.Context) (store.Loaded, error) {
	var ev store.Loaded
	me, err := c.Me(ctx)
	if err != nil {
		return ev, err
	}
	ev.Me = me
	var teams []wireTeam
	if err := c.do(ctx, "GET", "/users/me/teams", nil, nil, &teams); err != nil {
		return ev, err
	}
	for _, t := range teams {
		if t.DeleteAt == 0 {
			ev.Teams = append(ev.Teams, store.Team{ID: store.TeamID(t.ID), Name: t.Name, DisplayName: t.DisplayName})
		}
	}
	var channels []wireChannel
	if err := c.do(ctx, "GET", "/users/me/channels", nil, nil, &channels); err != nil {
		return ev, err
	}
	for _, ch := range channels {
		if ch.DeleteAt == 0 {
			ev.Channels = append(ev.Channels, ch.channel())
		}
	}
	// page=-1 streams every member as one JSON object a line.
	err = c.do(ctx, "GET", "/users/me/channel_members", url.Values{"page": {"-1"}}, nil, func(d *json.Decoder) error {
		for {
			var m wireMember
			if err := d.Decode(&m); err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}
			ev.Members = append(ev.Members, m.member())
		}
	})
	return ev, err
}

func (c *Client) Categories(ctx context.Context, team store.TeamID) ([]store.Category, error) {
	var w wireCategories
	if err := c.do(ctx, "GET", "/users/me/teams/"+string(team)+"/channels/categories", nil, nil, &w); err != nil {
		return nil, err
	}
	byID := map[string]wireCategory{}
	for _, cat := range w.Categories {
		byID[cat.ID] = cat
	}
	var out []store.Category
	for _, id := range w.Order {
		if cat, ok := byID[id]; ok {
			out = append(out, cat.category())
		}
	}
	return out, nil
}

func (c *Client) Channel(ctx context.Context, id store.ChannelID) (store.Channel, error) {
	var ch wireChannel
	err := c.do(ctx, "GET", "/channels/"+string(id), nil, nil, &ch)
	return ch.channel(), err
}

// MemberCount fetches how many members a channel has.
func (c *Client) MemberCount(ctx context.Context, id store.ChannelID) (int64, error) {
	var out struct {
		MemberCount int64 `json:"member_count"`
	}
	err := c.do(ctx, "GET", "/channels/"+string(id)+"/stats", nil, nil, &out)
	return out.MemberCount, err
}

func (c *Client) Member(ctx context.Context, id store.ChannelID) (store.ChannelMember, error) {
	var m wireMember
	err := c.do(ctx, "GET", "/channels/"+string(id)+"/members/me", nil, nil, &m)
	return m.member(), err
}

func (c *Client) Users(ctx context.Context, ids []store.UserID) ([]store.User, error) {
	var ws []wireUser
	if err := c.do(ctx, "POST", "/users/ids", nil, ids, &ws); err != nil {
		return nil, err
	}
	out := make([]store.User, len(ws))
	for i, u := range ws {
		out[i] = u.user()
	}
	return out, nil
}

// SearchUsers finds active users whose names start with a term, for
// autocomplete and for opening a DM.
func (c *Client) SearchUsers(ctx context.Context, term string) ([]store.User, error) {
	var ws []wireUser
	q := url.Values{"name": {term}, "limit": {"25"}}
	if err := c.do(ctx, "GET", "/users/autocomplete", q, nil, &struct {
		Users *[]wireUser `json:"users"`
	}{&ws}); err != nil {
		return nil, err
	}
	out := make([]store.User, len(ws))
	for i, u := range ws {
		out[i] = u.user()
	}
	return out, nil
}

func (c *Client) Statuses(ctx context.Context, ids []store.UserID) (map[store.UserID]store.Status, error) {
	var ws []wireStatus
	if err := c.do(ctx, "POST", "/users/status/ids", nil, ids, &ws); err != nil {
		return nil, err
	}
	out := make(map[store.UserID]store.Status, len(ws))
	for _, s := range ws {
		out[store.UserID(s.UserID)] = store.Status(s.Status)
	}
	return out, nil
}

// Posts fetches a page of a channel's posts before a post, or the newest
// page when before is empty.
func (c *Client) Posts(ctx context.Context, ch store.ChannelID, before store.PostID) (store.PostsLoaded, error) {
	// Collapsed Reply Threads, as the webapp: root posts only, with their
	// reply counts and participants.
	q := url.Values{"per_page": {strconv.Itoa(PageSize)}, "collapsedThreads": {"true"}}
	if before != "" {
		q.Set("before", string(before))
	}
	var l wirePostList
	if err := c.do(ctx, "GET", "/channels/"+string(ch)+"/posts", q, nil, &l); err != nil {
		return store.PostsLoaded{}, err
	}
	return store.PostsLoaded{Channel: ch, Posts: l.posts(), Older: before != "", HasOlder: len(l.Order) >= PageSize}, nil
}

func (c *Client) Thread(ctx context.Context, root store.PostID) (store.ThreadLoaded, error) {
	var l wirePostList
	if err := c.do(ctx, "GET", "/posts/"+string(root)+"/thread", nil, nil, &l); err != nil {
		return store.ThreadLoaded{}, err
	}
	return store.ThreadLoaded{Root: root, Posts: l.posts()}, nil
}

// NewPost is a post to create.
type NewPost struct {
	ChannelID store.ChannelID
	RootID    store.PostID
	Message   string
	FileIDs   []store.FileID
	PendingID string
}

func (c *Client) CreatePost(ctx context.Context, p NewPost) (store.Post, error) {
	body := map[string]any{"channel_id": p.ChannelID, "message": p.Message, "pending_post_id": p.PendingID}
	if p.RootID != "" {
		body["root_id"] = p.RootID
	}
	if len(p.FileIDs) > 0 {
		body["file_ids"] = p.FileIDs
	}
	var w wirePost
	err := c.do(ctx, "POST", "/posts", nil, body, &w)
	return w.post(), err
}

func (c *Client) EditPost(ctx context.Context, id store.PostID, message string) (store.Post, error) {
	var w wirePost
	err := c.do(ctx, "PUT", "/posts/"+string(id)+"/patch", nil, map[string]string{"message": message}, &w)
	return w.post(), err
}

func (c *Client) DeletePost(ctx context.Context, id store.PostID) error {
	return c.do(ctx, "DELETE", "/posts/"+string(id), nil, nil, nil)
}

func (c *Client) AddReaction(ctx context.Context, me store.UserID, post store.PostID, emoji string) error {
	return c.do(ctx, "POST", "/reactions", nil, map[string]string{"user_id": string(me), "post_id": string(post), "emoji_name": emoji}, nil)
}

func (c *Client) RemoveReaction(ctx context.Context, me store.UserID, post store.PostID, emoji string) error {
	return c.do(ctx, "DELETE", "/users/"+string(me)+"/posts/"+string(post)+"/reactions/"+url.PathEscape(emoji), nil, nil, nil)
}

// ViewChannel marks a channel read.
func (c *Client) ViewChannel(ctx context.Context, ch store.ChannelID) error {
	return c.do(ctx, "POST", "/channels/members/me/view", nil, map[string]any{"channel_id": ch, "collapsed_threads_supported": true}, nil)
}

// DirectChannel returns the DM with another user, creating it if needed.
func (c *Client) DirectChannel(ctx context.Context, me, other store.UserID) (store.Channel, error) {
	var ch wireChannel
	err := c.do(ctx, "POST", "/channels/direct", nil, []store.UserID{me, other}, &ch)
	return ch.channel(), err
}

type multipartBody struct {
	buf         *bytes.Buffer
	contentType string
}

func (c *Client) UploadFile(ctx context.Context, ch store.ChannelID, name string, data []byte) (store.FileInfo, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	_ = w.WriteField("channel_id", string(ch))
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		return store.FileInfo{}, err
	}
	if _, err := part.Write(data); err != nil {
		return store.FileInfo{}, err
	}
	if err := w.Close(); err != nil {
		return store.FileInfo{}, err
	}
	var out struct {
		FileInfos []wireFile `json:"file_infos"`
	}
	if err := c.do(ctx, "POST", "/files", nil, &multipartBody{buf, w.FormDataContentType()}, &out); err != nil {
		return store.FileInfo{}, err
	}
	if len(out.FileInfos) != 1 {
		return store.FileInfo{}, errors.New("the server stored no file")
	}
	return out.FileInfos[0].file(), nil
}

// FileKind is which bytes of a file to fetch.
type FileKind string

const (
	FileFull      FileKind = ""
	FileThumbnail FileKind = "/thumbnail"
	FilePreview   FileKind = "/preview"
)

func (c *Client) File(ctx context.Context, id store.FileID, kind FileKind) ([]byte, error) {
	var b []byte
	err := c.do(ctx, "GET", "/files/"+string(id)+string(kind), nil, nil, &b)
	return b, err
}

func (c *Client) ProfileImage(ctx context.Context, id store.UserID) ([]byte, error) {
	var b []byte
	err := c.do(ctx, "GET", "/users/"+string(id)+"/image", nil, nil, &b)
	return b, err
}

// WebImage fetches an image a post links to, as a link preview's. The
// session token goes only to the server itself, as its image proxy.
func (c *Client) WebImage(ctx context.Context, raw string) ([]byte, error) {
	u, err := c.base.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("not a web image: %s", raw)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	if u.Host == c.base.Host {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s answered %s", u.Host, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 16<<20))
}

// EmojiImageByName fetches the image of a custom emoji.
func (c *Client) EmojiImageByName(ctx context.Context, name string) ([]byte, error) {
	var w struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, "GET", "/emoji/name/"+url.PathEscape(name), nil, nil, &w); err != nil {
		return nil, err
	}
	var b []byte
	err := c.do(ctx, "GET", "/emoji/"+w.ID+"/image", nil, nil, &b)
	return b, err
}
