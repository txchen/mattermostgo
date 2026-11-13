package mm

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// fakeServer answers the API calls the client makes, as a Mattermost
// server shapes its answers (server/public/model in mattermost/mattermost).
type fakeServer struct {
	*httptest.Server
	t *testing.T

	mu       sync.Mutex
	mfa      bool
	token    string
	sockets  chan *serverSocket
	rejectWS bool
}

type serverSocket struct {
	conn net.Conn
	br   *bufio.Reader
}

// send writes a text frame, unmasked as servers send them.
func (s *serverSocket) send(v any) error {
	b, _ := json.Marshal(v)
	frame := []byte{0x81}
	switch n := len(b); {
	case n < 126:
		frame = append(frame, byte(n))
	default:
		frame = append(frame, 126, byte(n>>8), byte(n))
	}
	_, err := s.conn.Write(append(frame, b...))
	return err
}

func (s *serverSocket) read() ([]byte, error) {
	c := &wsConn{rw: s.conn, br: s.br}
	_, _, payload, err := c.readFrame()
	return payload, err
}

func newFakeServer(t *testing.T) *fakeServer {
	f := &fakeServer{t: t, token: "tok", sockets: make(chan *serverSocket, 4)}
	mux := http.NewServeMux()
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+f.token {
				w.WriteHeader(401)
				fmt.Fprint(w, `{"id":"api.context.session_expired.app_error","message":"Invalid or expired session, please login again.","status_code":401}`)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/v4/system/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"OK"}`)
	})
	mux.HandleFunc("POST /api/v4/users/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		switch {
		case body["login_id"] != "alice" || body["password"] != "pw":
			w.WriteHeader(401)
			fmt.Fprint(w, `{"id":"api.user.login.invalid_credentials_email_username","message":"Enter a valid email or username and/or password.","status_code":401}`)
		case f.mfa && body["token"] == "":
			w.WriteHeader(400)
			fmt.Fprint(w, `{"id":"mfa.validate_token.authenticate.app_error","message":"Invalid MFA token.","status_code":400}`)
		case f.mfa && body["token"] != "123456":
			w.WriteHeader(401)
			fmt.Fprint(w, `{"id":"api.user.check_user_mfa.bad_code.app_error","message":"Invalid MFA token.","status_code":401}`)
		default:
			w.Header().Set("Token", f.token)
			fmt.Fprint(w, `{"id":"alice1","username":"alice","first_name":"Alice","last_name":"Liddell"}`)
		}
	})
	mux.HandleFunc("GET /api/v4/users/me", authed(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"alice1","username":"alice"}`)
	}))
	mux.HandleFunc("GET /api/v4/users/me/teams", authed(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":"t1","name":"eng","display_name":"Engineering","delete_at":0},{"id":"t2","name":"old","display_name":"Old","delete_at":5}]`)
	}))
	mux.HandleFunc("GET /api/v4/users/me/channels", authed(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":"c1","team_id":"t1","type":"O","name":"town-square","display_name":"Town Square","total_msg_count":12,"last_post_at":100,"delete_at":0},
{"id":"d1","team_id":"","type":"D","name":"alice1__bob1","display_name":"","total_msg_count":3,"delete_at":0}]`)
	}))
	mux.HandleFunc("GET /api/v4/users/me/channel_members", authed(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "-1" {
			t.Errorf("channel members page %q", r.URL.Query().Get("page"))
		}
		fmt.Fprint(w, `{"channel_id":"c1","user_id":"alice1","msg_count":10,"mention_count":1,"last_viewed_at":90,"notify_props":{"desktop":"default","mark_unread":"all"}}
{"channel_id":"d1","user_id":"alice1","msg_count":3,"mention_count":0,"notify_props":{"desktop":"all","mark_unread":"mention"}}
`)
	}))
	mux.HandleFunc("GET /api/v4/users/me/teams/t1/channels/categories", authed(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"categories":[{"id":"cat-dm","team_id":"t1","type":"direct_messages","display_name":"Direct Messages","channel_ids":["d1"]},
{"id":"cat-ch","team_id":"t1","type":"channels","display_name":"Channels","channel_ids":["c1"]}],"order":["cat-ch","cat-dm"]}`)
	}))
	mux.HandleFunc("GET /api/v4/channels/c1/posts", authed(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("before") != "" {
			fmt.Fprint(w, `{"order":["p0"],"posts":{"p0":{"id":"p0","channel_id":"c1","user_id":"bob1","create_at":1,"message":"first"}}}`)
			return
		}
		fmt.Fprint(w, `{"order":["p2","p1","gone"],"posts":{
"p2":{"id":"p2","channel_id":"c1","user_id":"bob1","create_at":20,"edit_at":25,"message":"**hi** @alice","props":{"from_bot":"true"},
 "metadata":{"files":[{"id":"f1","name":"cat.png","extension":"png","mime_type":"image/png","size":2048,"width":640,"height":480,"has_preview_image":true}],
 "reactions":[{"user_id":"alice1","post_id":"p2","emoji_name":"+1","create_at":30}]}},
"p1":{"id":"p1","channel_id":"c1","user_id":"alice1","create_at":10,"message":"hello"},
"gone":{"id":"gone","channel_id":"c1","user_id":"alice1","create_at":15,"delete_at":16,"message":""}},"prev_post_id":"p0"}`)
	}))
	mux.HandleFunc("POST /api/v4/posts", authed(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"id": "new1", "channel_id": body["channel_id"], "user_id": "alice1", "create_at": 50, "message": body["message"], "pending_post_id": body["pending_post_id"]})
	}))
	mux.HandleFunc("GET /api/v4/websocket", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		reject := f.rejectWS
		f.mu.Unlock()
		if reject || r.Header.Get("Authorization") != "Bearer "+f.token {
			w.WriteHeader(401)
			return
		}
		conn, brw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		fmt.Fprintf(brw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
		brw.Flush()
		f.sockets <- &serverSocket{conn: conn, br: brw.Reader}
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeServer) base() *url.URL {
	u, _ := url.Parse(f.URL)
	return u
}
