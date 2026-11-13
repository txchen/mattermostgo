// Seed fills an empty Mattermost server with teams, users, channels and
// posts for manual and end-to-end testing.
//
//	go run ./scripts/seed -url http://localhost:8065
//
// Every user's password is Password1!. The MFA secret of mfauser is
// written to the file -mfa names.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const password = "Password1!"

var base string

type client struct{ token string }

func (c *client) call(method, path string, body any, out any) {
	var r io.Reader
	ct := "application/json"
	switch b := body.(type) {
	case nil:
	case *bytes.Buffer:
		r = b
	case multipartBody:
		r, ct = b.buf, b.ct
	default:
		j, _ := json.Marshal(b)
		r = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, base+"/api/v4"+path, r)
	req.Header.Set("Content-Type", ct)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		log.Fatalf("%s %s: %s %s", method, path, res.Status, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			log.Fatalf("%s %s: %v", method, path, err)
		}
	}
	if c.token == "" && res.Header.Get("Token") != "" {
		c.token = res.Header.Get("Token")
	}
}

type multipartBody struct {
	buf *bytes.Buffer
	ct  string
}

type obj = map[string]any

func login(username string) *client {
	c := &client{}
	c.call("POST", "/users/login", obj{"login_id": username, "password": password}, nil)
	return c
}

func id(m obj) string { return m["id"].(string) }

// totp is the RFC 6238 code of a base32 secret at a time.
func totp(secret string, at time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(secret, "=")))
	if err != nil {
		log.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(at.Unix()/30))
	h := hmac.New(sha1.New, key)
	h.Write(msg[:])
	sum := h.Sum(nil)
	o := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[o:o+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1000000)
}

// bigMessages are the shapes of the posts of ~big, in turn.
var bigMessages = []string{
	"Post %d: a plain line of text, as most posts are.",
	"Post %d with **bold**, _italic_, `code` and a [link](https://mattermost.com) :tada:",
	"Post %d asks @alice and @bob to look at ~dev :eyes:",
	"Post %d has a list:\n\n- one\n- two\n- three",
	"Post %d has code:\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
	"Post %d is longer. Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.",
	"Post %d 中文消息，混合 English 和 emoji :rocket:",
	"| post | %d |\n|---|---|\n| a | b |",
}

func seedBig(n int) {
	admin := login("admin")
	var team, me obj
	admin.call("GET", "/teams/name/eng", nil, &team)
	var ch obj
	admin.call("POST", "/channels", obj{"team_id": id(team), "name": "big", "display_name": "Big Channel", "type": "O", "purpose": fmt.Sprint(n, " posts for scrolling")}, &ch)
	posters := []*client{login("bob"), login("carol"), login("alice")}
	for _, u := range []string{"alice", "bob", "carol"} {
		admin.call("GET", "/users/username/"+u, nil, &me)
		admin.call("POST", "/channels/"+id(ch)+"/members", obj{"user_id": id(me)}, nil)
	}
	var last string
	for i := range n {
		body := obj{"channel_id": id(ch), "message": fmt.Sprintf(bigMessages[i%len(bigMessages)], i+1)}
		if i%40 == 39 && last != "" {
			body["root_id"] = last
		}
		var out obj
		posters[i%3].call("POST", "/posts", body, &out)
		if i%40 == 0 {
			last = id(out)
			posters[i%3].call("POST", "/reactions", obj{"user_id": out["user_id"], "post_id": id(out), "emoji_name": "+1"}, nil)
		}
	}
	fmt.Println("seeded ~big with", n, "posts")
}

func main() {
	flag.StringVar(&base, "url", "http://localhost:8065", "the server")
	mfaFile := flag.String("mfa", "", "where to write mfauser's MFA secret")
	big := flag.Int("big", 0, "only add a channel ~big with this many posts, to a seeded server")
	flag.Parse()
	if *big > 0 {
		seedBig(*big)
		return
	}

	anon := &client{}
	users := map[string]obj{}
	for _, u := range []string{"admin", "alice", "bob", "carol", "mfauser"} {
		var out obj
		anon.call("POST", "/users", obj{"username": u, "email": u + "@example.com", "password": password, "first_name": strings.ToUpper(u[:1]) + u[1:], "last_name": "Test"}, &out)
		users[u] = out
	}
	admin := login("admin")
	teams := map[string]obj{}
	for _, t := range [][2]string{{"eng", "Engineering"}, {"ops", "Operations"}} {
		var out obj
		admin.call("POST", "/teams", obj{"name": t[0], "display_name": t[1], "type": "O"}, &out)
		teams[t[0]] = out
		for _, u := range []string{"alice", "bob", "carol", "mfauser"} {
			admin.call("POST", "/teams/"+id(out)+"/members", obj{"team_id": id(out), "user_id": id(users[u])}, nil)
		}
	}
	eng := id(teams["eng"])
	var town obj
	admin.call("GET", "/teams/"+eng+"/channels/name/town-square", nil, &town)
	channels := map[string]string{"town-square": id(town)}
	for _, ch := range []struct{ name, display, kind string }{{"dev", "Developers", "O"}, {"secret", "Secret Plans", "P"}, {"random", "Random", "O"}} {
		var out obj
		admin.call("POST", "/channels", obj{"team_id": eng, "name": ch.name, "display_name": ch.display, "type": ch.kind, "purpose": "Where " + strings.ToLower(ch.display) + " talk"}, &out)
		channels[ch.name] = id(out)
		for _, u := range []string{"alice", "bob", "carol"} {
			admin.call("POST", "/channels/"+id(out)+"/members", obj{"user_id": id(users[u])}, nil)
		}
	}
	bob, carol, alice := login("bob"), login("carol"), login("alice")
	post := func(c *client, ch, msg, root string) string {
		var out obj
		body := obj{"channel_id": ch, "message": msg}
		if root != "" {
			body["root_id"] = root
		}
		c.call("POST", "/posts", body, &out)
		return id(out)
	}
	town1 := post(bob, channels["town-square"], "Welcome to **Mattermost Go** testing :wave:\n\n- lists\n- `code`\n\n```go\nfmt.Println(\"hi\")\n```", "")
	post(carol, channels["town-square"], "Thanks @bob! Ping @alice, see ~dev :rocket:", "")
	post(bob, channels["town-square"], "Replying in a thread", town1)
	post(alice, channels["town-square"], "| a | b |\n|---|---|\n| 1 | 2 |", "")
	bob.call("POST", "/reactions", obj{"user_id": id(users["bob"]), "post_id": town1, "emoji_name": "+1"}, nil)
	carol.call("POST", "/reactions", obj{"user_id": id(users["carol"]), "post_id": town1, "emoji_name": "+1"}, nil)
	for i := range 130 {
		post([]*client{bob, carol}[i%2], channels["dev"], fmt.Sprintf("Dev chatter number %d", i+1), "")
	}
	post(bob, channels["dev"], "@alice can you review?", "")

	var dm obj
	bob.call("POST", "/channels/direct", []string{id(users["bob"]), id(users["alice"])}, &dm)
	post(bob, id(dm), "Hey Alice, a direct message", "")

	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := range 200 {
		for x := range 320 {
			img.Set(x, y, color.RGBA{uint8(x * 255 / 320), uint8(y * 255 / 200), 160, 255})
		}
	}
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	w.WriteField("channel_id", channels["random"])
	part, _ := w.CreateFormFile("files", "gradient.png")
	part.Write(pngBuf.Bytes())
	w.Close()
	var up struct {
		FileInfos []obj `json:"file_infos"`
	}
	carol.call("POST", "/files", multipartBody{buf, w.FormDataContentType()}, &up)
	carol.call("POST", "/posts", obj{"channel_id": channels["random"], "message": "A picture", "file_ids": []string{id(up.FileInfos[0])}}, nil)

	mfa := login("mfauser")
	var gen struct {
		Secret string `json:"secret"`
	}
	mfa.call("POST", "/users/"+id(users["mfauser"])+"/mfa/generate", nil, &gen)
	mfa.call("PUT", "/users/"+id(users["mfauser"])+"/mfa", obj{"activate": true, "code": totp(gen.Secret, time.Now())}, nil)
	if *mfaFile != "" {
		os.WriteFile(*mfaFile, []byte(gen.Secret), 0o600)
	}
	fmt.Println("seeded", base)
}
