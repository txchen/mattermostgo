// Package e2e drives the client against a real Mattermost server seeded
// by scripts/seed. It skips unless MM_E2E_URL names the server:
//
//	MM_E2E_URL=http://localhost:8065 MM_E2E_MFA=/path/to/mfa-secret go test ./internal/e2e
//
// MM_FRAMES=dir saves what the window shows at each step as PNGs.
package e2e

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/media"
	"github.com/txchen/mattermostgo/internal/store"
	"github.com/txchen/mattermostgo/internal/view"
)

type memSecrets struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memSecrets) Get(k string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], nil
}
func (s *memSecrets) Set(k string, v []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *memSecrets) Delete(k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}

type client struct {
	t     *testing.T
	a     *app.App
	v     *view.View
	tt    *ui.Tester
	mu    sync.Mutex
	queue []func()
	step  int
}

func newClient(t *testing.T) *client {
	c := &client{t: t}
	c.a = &app.App{
		Run:        func(f func()) { c.mu.Lock(); c.queue = append(c.queue, f); c.mu.Unlock() },
		HC:         http.DefaultClient,
		Secrets:    &memSecrets{m: map[string][]byte{}},
		ConfigPath: filepath.Join(t.TempDir(), "servers.json"),
	}
	c.v = view.New(c.a, media.New(c.a.Run, t.TempDir(), 32<<20))
	c.a.Error = func(msg string) { t.Logf("app error: %s", msg) }
	if err := c.a.Start(); err != nil {
		t.Fatal(err)
	}
	c.tt = ui.NewTester(c.v.Build, 1200, 800)
	t.Cleanup(func() {
		for _, s := range c.a.Servers {
			c.a.Logout(s)
		}
	})
	return c
}

// until runs what comes back to the main thread, drawing frames, until
// cond holds.
func (c *client) until(what string, cond func() bool) {
	c.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		c.mu.Lock()
		q := c.queue
		c.queue = nil
		c.mu.Unlock()
		for _, f := range q {
			f()
		}
		c.tt.Frame()
		if cond() {
			c.snapshot(what)
			return
		}
		if time.Now().After(deadline) {
			c.snapshot("timeout " + what)
			c.t.Fatalf("%s: timed out; texts %q", what, c.tt.Texts())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (c *client) snapshot(name string) {
	dir := os.Getenv("MM_FRAMES")
	if dir == "" {
		return
	}
	c.step++
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%02d-%s.png", c.t.Name(), c.step, strings.ReplaceAll(name, " ", "-"))))
	if err != nil {
		c.t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, c.tt.Image())
}

func (c *client) has(s string) func() bool {
	return func() bool { return strings.Contains(strings.Join(c.tt.Texts(), "\n"), s) }
}

func (c *client) click(label string) {
	c.t.Helper()
	if err := c.tt.Click(label); err != nil {
		c.t.Fatalf("click %q: %v in %q", label, err, c.tt.Texts())
	}
}

func serverURL(t *testing.T) string {
	u := os.Getenv("MM_E2E_URL")
	if u == "" {
		t.Skip("MM_E2E_URL not set")
	}
	return u
}

// api calls the server as another user, as the people the user talks with.
func api(t *testing.T, base, user, method, path string, body any) map[string]any {
	t.Helper()
	call := func(token, method, path string, body any) (*http.Response, map[string]any) {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, base+"/api/v4"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		json.NewDecoder(res.Body).Decode(&out)
		if res.StatusCode >= 300 {
			t.Fatalf("%s %s: %s %v", method, path, res.Status, out)
		}
		return res, out
	}
	res, _ := call("", "POST", "/users/login", map[string]string{"login_id": user, "password": "Password1!"})
	_, out := call(res.Header.Get("Token"), method, path, body)
	return out
}

// The everyday flow: add a server, log in, read the channels, send a
// post, and get others' posts, reactions and typing as they happen.
func TestEverydayChat(t *testing.T) {
	base := serverURL(t)
	var devCh map[string]any
	team := api(t, base, "bob", "GET", "/teams/name/eng", nil)
	devCh = api(t, base, "bob", "GET", "/teams/"+team["id"].(string)+"/channels/name/dev", nil)
	api(t, base, "bob", "POST", "/posts", map[string]any{"channel_id": devCh["id"], "message": "@alice can you review?"})
	townCh := api(t, base, "bob", "GET", "/teams/"+team["id"].(string)+"/channels/name/town-square", nil)
	hello := fmt.Sprintf("Good morning **everyone** %d", time.Now().UnixNano())
	api(t, base, "bob", "POST", "/posts", map[string]any{"channel_id": townCh["id"], "message": hello})
	c := newClient(t)
	c.until("welcome", c.has("Connect to your Mattermost server"))
	c.tt.Type(base)
	c.click("Add Server")
	c.until("login form", c.has("Log in to"))

	c.tt.Type("alice")
	c.tt.Key(0, ui.KeyTab)
	c.tt.Type("Password1!")
	c.tt.Key(0, ui.KeyEnter)
	c.until("town square", c.has(strings.ReplaceAll(strings.TrimPrefix(hello, "Good morning **everyone** "), "**", "")))
	s := c.a.Current
	for _, want := range []string{"Developers", "Secret Plans", "bob", "CHANNELS", "DIRECT MESSAGES", "Good morning", "everyone"} {
		if !c.has(want)() {
			t.Errorf("no %q after login: %q", want, c.tt.Texts())
		}
	}
	if s.Store.Mentions(channelByName(s, "dev")) == 0 {
		t.Error("no mention counted in ~dev")
	}

	c.click("Developers")
	c.until("dev channel", c.has("can you review?"))
	if s.Store.Mentions(channelByName(s, "dev")) != 0 {
		t.Error("opening ~dev did not clear its mentions")
	}
	dev := channelByName(s, "dev")
	c.until("ws online", func() bool { _, ok := s.Store.Conn.(store.ConnOnline); return ok })

	msg := fmt.Sprintf("hello from the native client %d", time.Now().UnixNano())
	c.tt.Type(msg)
	c.tt.Key(0, ui.KeyEnter)
	c.until("sent", func() bool {
		tl := s.Store.Timelines[dev]
		p := s.Store.Posts[tl.Order[len(tl.Order)-1]]
		return p.Message == msg && p.State == store.Sent && !strings.Contains(string(p.ID), ":")
	})

	api(t, base, "bob", "POST", "/posts", map[string]any{"channel_id": dev, "message": "real time from bob :tada:"})
	c.until("bob's post", c.has("real time from bob 🎉"))

	tl := s.Store.Timelines[dev]
	last := tl.Order[len(tl.Order)-1]
	api(t, base, "carol", "POST", "/reactions", map[string]any{"user_id": userByName(s, "carol"), "post_id": string(last), "emoji_name": "heart"})
	c.until("reaction", func() bool { return len(s.Store.Posts[last].Reactions) == 1 })

	town := channelByName(s, "town-square")
	api(t, base, "bob", "POST", "/posts", map[string]any{"channel_id": town, "message": "unseen in town"})
	c.until("unread town", func() bool { return s.Store.Unread(town) })

	c.tt.Key(ui.Cmd, ui.KeyK)
	c.tt.Type("random")
	c.tt.Key(0, ui.KeyEnter)
	c.until("random with image", func() bool { return s.Channel == channelByName(s, "random") && c.has("A picture")() })
	settled := time.Now().Add(time.Second)
	c.until("images loaded", func() bool { return time.Now().After(settled) })
}

// Older posts load as the list scrolls up.
func TestInfiniteScroll(t *testing.T) {
	base := serverURL(t)
	c := newClient(t)
	c.a.AddServer("Local", base, func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	})
	c.until("added", func() bool { return len(c.a.Servers) == 1 })
	c.a.Login(c.a.Current, "alice", "Password1!", "")
	c.until("ready", func() bool { return c.a.Current.Ready })
	s := c.a.Current
	dev := channelByName(s, "dev")
	c.a.OpenChannel(s, dev)
	c.until("first page", func() bool { t := s.Store.Timelines[dev]; return t != nil && len(t.Order) > 0 && !t.Loading })
	first := len(s.Store.Timelines[dev].Order)
	if first != 60 {
		t.Errorf("opening loaded %d posts, not one page", first)
	}
	for range 20 {
		c.tt.Scroll(600, 300, 0, -3000)
		c.until("scrolled", func() bool { return !s.Store.Timelines[dev].Loading })
	}
	if tl := s.Store.Timelines[dev]; len(tl.Order) <= first || tl.HasOlder {
		t.Errorf("%d posts after scrolling to the top, more to load: %v", len(tl.Order), tl.HasOlder)
	}
	c.until("the first post", c.has("Dev chatter number 1\n"))
}

// An account with MFA asks for the code before it logs in.
func TestMFALogin(t *testing.T) {
	base := serverURL(t)
	path := os.Getenv("MM_E2E_MFA")
	if path == "" {
		t.Skip("MM_E2E_MFA not set")
	}
	secret, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c := newClient(t)
	c.a.AddServer("Local", base, func(err error) {})
	c.until("login form", c.has("Log in to"))
	c.tt.Type("mfauser")
	c.tt.Key(0, ui.KeyTab)
	c.tt.Type("Password1!")
	c.tt.Key(0, ui.KeyEnter)
	c.until("MFA step", c.has("authenticator"))
	c.tt.Type("000000")
	c.tt.Key(0, ui.KeyEnter)
	c.until("wrong code", c.has("Invalid MFA token"))
	// The server takes each code once: a run in the window of the last
	// waits for the next.
	used := path + ".window"
	if last, _ := os.ReadFile(used); string(last) == fmt.Sprint(time.Now().Unix()/30) {
		time.Sleep(time.Duration(30-time.Now().Unix()%30+1) * time.Second)
	}
	os.WriteFile(used, []byte(fmt.Sprint(time.Now().Unix()/30)), 0o600)
	c.tt.Type(totp(string(secret), time.Now()))
	c.tt.Key(0, ui.KeyEnter)
	c.until("logged in", func() bool { return c.a.Current.Ready })
}

func channelByName(s *app.Server, name string) store.ChannelID {
	for id, ch := range s.Store.Channels {
		if ch.Name == name && (ch.TeamID == s.Team || ch.TeamID == "") {
			return id
		}
	}
	return ""
}

func userByName(s *app.Server, name string) string {
	for id, u := range s.Store.Users {
		if u.Username == name {
			return string(id)
		}
	}
	return ""
}

func totp(secret string, at time.Time) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(strings.TrimSpace(secret), "=")))
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(at.Unix()/30))
	h := hmac.New(sha1.New, key)
	h.Write(msg[:])
	sum := h.Sum(nil)
	o := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[o:o+4])&0x7fffffff)%1000000)
}
