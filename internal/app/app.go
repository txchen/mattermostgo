// Package app wires servers, their stores and the network together. Its
// fields and methods belong to the main thread; work on other goroutines
// comes back through Run.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/txchen/mattermostgo/internal/config"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// Session is the login state of a server.
type Session interface{ isSession() }

type LoggedOut struct{ Err string }

// NeedsMFA holds the password typed, for the second step of the login.
type NeedsMFA struct {
	LoginID, Password string
	Err               string
}

type LoggingIn struct{ Prev Session }

type LoggedIn struct{ Client *mm.Client }

func (LoggedOut) isSession() {}
func (NeedsMFA) isSession()  {}
func (LoggingIn) isSession() {}
func (LoggedIn) isSession()  {}

// Server is a server the user added, with its session and data.
type Server struct {
	Config  config.Server
	URL     *url.URL
	Session Session
	Store   *store.Store
	// Ready is true once the first load after login arrived.
	Ready bool

	Team    store.TeamID
	Channel store.ChannelID
	// Thread is the root post shown in the RHS, if any.
	Thread store.PostID
	// ThreadsView shows the followed threads in place of the channel.
	ThreadsView bool
	// Jump is the post last jumped to, which the view scrolls to once and
	// highlights for a while.
	Jump Jump
	// NewSince is when the open channel was last viewed before opening
	// it, where the line of new posts goes; 0 when all was read.
	NewSince int64
	// History of opened channels, for Cmd+[ and Cmd+].
	back, forward []store.ChannelID

	cancel  context.CancelFunc
	socket  *mm.Socket
	seq     int
	asked   map[store.UserID]bool
	typedAt time.Time
	// commands are the slash commands of each team, and emojis the
	// custom emoji by name, loaded once.
	commands map[store.TeamID][]mm.AutocompleteCommand
	emojis   map[string]string
}

// Client returns the server's API client while logged in.
func (s *Server) Client() *mm.Client {
	if in, ok := s.Session.(LoggedIn); ok {
		return in.Client
	}
	return nil
}

// Me returns the current user, once loaded.
func (s *Server) Me() *store.User { return s.Store.Users[s.Store.Me] }

// Notification is a desktop notification to show.
type Notification struct {
	Server  config.ServerID
	Channel store.ChannelID
	Title   string
	Body    string
	// Silent leaves out the sound, as the settings may ask.
	Silent bool
}

type App struct {
	Servers []*Server
	Current *Server

	// Run runs a function on the main thread and draws a frame.
	Run        func(func())
	HC         *http.Client
	Secrets    config.Secrets
	ConfigPath string
	// Focused reports whether the window has the user's attention.
	Focused func() bool
	Notify  func(Notification)
	Badge   func(mentions int64, unread bool)
	// Error shows an error that belongs to no form.
	Error func(msg string)
	// OpenExternal opens a link in the browser, and Reveal a file in Finder.
	OpenExternal func(url string)
	Reveal       func(path string)
	// QuickLook shows a file in the Quick Look panel, and OpenPath opens
	// it in its app.
	QuickLook func(path string) error
	OpenPath  func(path string) error

	// Theme names the look of the window, empty to follow the system.
	Theme string

	last map[config.ServerID]config.Place
}

// Start reads the servers and logs into those with a saved token.
func (a *App) Start() error {
	f, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	a.last = f.Last
	a.Theme = f.Theme
	if a.last == nil {
		a.last = map[config.ServerID]config.Place{}
	}
	for _, sc := range f.Servers {
		u, err := mm.ParseServerURL(sc.URL)
		if err != nil {
			continue
		}
		s := &Server{Config: sc, URL: u, Session: LoggedOut{}, Store: store.New()}
		a.Servers = append(a.Servers, s)
		if sc.ID == f.Current || a.Current == nil {
			a.Current = s
		}
		// The Keychain may ask the user before it answers, so it is read
		// off the main thread.
		s.Session = LoggingIn{Prev: LoggedOut{}}
		goRun(a, func() ([]byte, error) { return a.Secrets.Get(config.TokenKey(sc.ID)) }, func(tok []byte, err error) {
			if _, waiting := s.Session.(LoggingIn); !waiting {
				return
			}
			if err != nil || len(tok) == 0 {
				s.Session = LoggedOut{}
				return
			}
			s.Session = LoggedIn{Client: mm.New(u, string(tok), a.HC)}
			a.connect(s)
		})
	}
	return nil
}

func (a *App) save() {
	f := config.File{Last: a.last, Theme: a.Theme}
	for _, s := range a.Servers {
		f.Servers = append(f.Servers, s.Config)
	}
	if a.Current != nil {
		f.Current = a.Current.Config.ID
	}
	if err := config.Save(a.ConfigPath, f); err != nil {
		log.Printf("saving servers: %v", err)
	}
}

// goRun runs work off the main thread and passes its result back to it.
func goRun[T any](a *App, work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		a.Run(func() { done(v, err) })
	}()
}

// AddServer checks that a URL is a Mattermost server and adds it.
func (a *App) AddServer(name, rawURL string, done func(error)) {
	u, err := mm.ParseServerURL(rawURL)
	if err != nil {
		done(err)
		return
	}
	for _, s := range a.Servers {
		if s.URL.String() == u.String() {
			done(fmt.Errorf("%s is added already, as %s", u, s.Config.Name))
			return
		}
	}
	goRun(a, func() (struct{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return struct{}{}, mm.Ping(ctx, a.HC, u)
	}, func(_ struct{}, err error) {
		if err != nil {
			done(fmt.Errorf("no Mattermost server answers at %s: %w", u, err))
			return
		}
		if strings.TrimSpace(name) == "" {
			name = u.Host
		}
		s := &Server{Config: config.Server{ID: config.NewServerID(), Name: strings.TrimSpace(name), URL: u.String()}, URL: u, Session: LoggedOut{}, Store: store.New()}
		a.Servers = append(a.Servers, s)
		a.Current = s
		a.save()
		done(nil)
	})
}

// SetTheme chooses the look of the window.
func (a *App) SetTheme(name string) {
	a.Theme = name
	a.save()
}

func (a *App) EditServer(s *Server, name string) {
	if name = strings.TrimSpace(name); name != "" {
		s.Config.Name = name
		a.save()
	}
}

func (a *App) RemoveServer(s *Server) {
	a.Logout(s)
	a.Servers = slices.DeleteFunc(a.Servers, func(x *Server) bool { return x == s })
	delete(a.last, s.Config.ID)
	if a.Current == s {
		a.Current = nil
		if len(a.Servers) > 0 {
			a.Current = a.Servers[0]
		}
	}
	a.save()
	a.updateBadge()
}

func (a *App) SelectServer(s *Server) {
	a.Current = s
	a.save()
	if s.Channel != "" {
		a.markViewed(s, s.Channel)
	}
}

// Login logs into a server, with the MFA code on the second step.
func (a *App) Login(s *Server, loginID, password, mfaCode string) {
	prev := s.Session
	s.Session = LoggingIn{Prev: prev}
	goRun(a, func() (*mm.Client, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c, _, err := mm.Login(ctx, a.HC, s.URL, loginID, password, mfaCode)
		return c, err
	}, func(c *mm.Client, err error) {
		switch {
		case errors.Is(err, mm.ErrMFARequired):
			s.Session = NeedsMFA{LoginID: loginID, Password: password}
		case err != nil:
			if _, mfa := prev.(NeedsMFA); mfa {
				s.Session = NeedsMFA{LoginID: loginID, Password: password, Err: err.Error()}
			} else {
				s.Session = LoggedOut{Err: err.Error()}
			}
		default:
			key, token := config.TokenKey(s.Config.ID), []byte(c.Token())
			go func() {
				if err := a.Secrets.Set(key, token); err != nil {
					log.Printf("saving the session token: %v", err)
				}
			}()
			s.Session = LoggedIn{Client: c}
			a.connect(s)
		}
	})
}

// Logout ends the session on the server and forgets the token.
func (a *App) Logout(s *Server) {
	if c := s.Client(); c != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = c.Logout(ctx)
		}()
	}
	a.endSession(s, LoggedOut{})
}

func (a *App) endSession(s *Server, next Session) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	key := config.TokenKey(s.Config.ID)
	go func() { _ = a.Secrets.Delete(key) }()
	s.Session = next
	s.Store = store.New()
	s.Ready = false
	s.socket = nil
	s.Team, s.Channel, s.Thread, s.ThreadsView = "", "", "", false
	s.commands, s.emojis = nil, nil
	s.back, s.forward = nil, nil
	a.updateBadge()
}

// connect loads a logged-in server and keeps its WebSocket open.
func (a *App) connect(s *Server) {
	c := s.Client()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.asked = map[store.UserID]bool{}
	st := s.Store
	apply := func(ev store.Event) {
		a.Run(func() {
			if s.Store == st {
				a.apply(s, ev)
			}
		})
	}
	expired := func() {
		a.Run(func() {
			if s.Store == st {
				a.endSession(s, LoggedOut{Err: "Your session expired. Log in again."})
			}
		})
	}
	reload := func(reconnected bool) {
		ev, err := c.Load(ctx)
		if errors.Is(err, mm.ErrUnauthorized) {
			expired()
			return
		}
		if err != nil {
			if ctx.Err() == nil {
				a.Run(func() { a.showError("Loading " + s.Config.Name + ": " + err.Error()) })
			}
			return
		}
		apply(ev)
		for _, t := range ev.Teams {
			if cats, err := c.Categories(ctx, t.ID); err == nil {
				apply(store.CategoriesLoaded{Team: t.ID, Categories: cats})
			}
		}
		a.Run(func() {
			if s.Store != st {
				return
			}
			first := !s.Ready
			s.Ready = true
			a.loadDMUsers(s)
			for _, t := range s.Store.TeamOrder {
				me, team := s.Store.Me, t
				Call(a, s, func(ctx context.Context, c *mm.Client) (store.ThreadsLoaded, error) { return c.Threads(ctx, me, team) },
					func(ev store.ThreadsLoaded) { a.apply(s, ev) })
			}
			if first {
				a.restorePlace(s)
			} else if reconnected && s.Channel != "" {
				a.fetchPosts(s, s.Channel, "")
				if s.Thread != "" {
					a.OpenThread(s, s.Thread)
				}
			}
		})
	}
	s.socket = &mm.Socket{Client: c, HC: a.HC, Me: "", Emit: apply, Expired: expired,
		Online: func(reconnected bool) {
			if reconnected {
				reload(true)
			}
		}}
	go func() {
		me, err := c.Me(ctx)
		if errors.Is(err, mm.ErrUnauthorized) {
			expired()
			return
		}
		if err == nil {
			s.socket.Me = me.ID
		}
		reload(false)
		s.socket.Run(ctx)
	}()
}

// apply applies an event from the network, then does what follows it.
func (a *App) apply(s *Server, ev store.Event) {
	var notify *store.Post
	mentioned := false
	if pc, ok := ev.(store.PostCreated); ok {
		if _, known := s.Store.Posts[pc.Post.ID]; !known {
			p := pc.Post
			notify, mentioned = &p, pc.Mentioned
		}
	}
	s.Store.Apply(ev)
	switch ev := ev.(type) {
	case store.PostCreated:
		a.askUsers(s, []store.UserID{ev.Post.UserID})
		a.showConversation(s, ev.Post.ChannelID)
		if ev.Post.ChannelID == s.Channel && a.Current == s && a.focused() && ev.Post.UserID != s.Store.Me {
			a.markViewed(s, s.Channel)
		}
	case store.PostsLoaded:
		var ids []store.UserID
		for _, p := range ev.Posts {
			ids = append(ids, p.UserID)
			ids = append(ids, p.Participants...)
		}
		a.askUsers(s, ids)
	case store.ThreadLoaded:
		var ids []store.UserID
		for _, p := range ev.Posts {
			ids = append(ids, p.UserID)
		}
		a.askUsers(s, ids)
	case store.ChannelRemoved:
		if s.Channel == ev.Channel {
			s.Channel, s.Thread = "", ""
			a.openDefault(s)
		}
	case store.ChannelUpserted:
		if ev.Channel.Type == store.Direct {
			a.askUsers(s, []store.UserID{ev.Channel.DMPartner(s.Store.Me)})
		}
	case store.Typing:
		a.askUsers(s, []store.UserID{ev.User})
	}
	if notify != nil && a.Notify != nil {
		ch, m := s.Store.Channels[notify.ChannelID], s.Store.Members[notify.ChannelID]
		viewing := a.Current == s && s.Channel == notify.ChannelID && a.focused()
		if shouldNotify(s.Me(), m, ch, notify, mentioned, viewing) {
			silent := s.Me() != nil && s.Me().DesktopSound == "false"
			a.Notify(Notification{Server: s.Config.ID, Channel: ch.ID, Title: a.ChannelName(s, ch), Body: a.notificationBody(s, notify), Silent: silent})
		}
	}
	a.updateBadge()
}

func (a *App) focused() bool { return a.Focused == nil || a.Focused() }

func (a *App) showError(msg string) {
	if a.Error != nil {
		a.Error(msg)
	} else {
		log.Print(msg)
	}
}

func (a *App) updateBadge() {
	if a.Badge == nil {
		return
	}
	stores := make([]*store.Store, len(a.Servers))
	for i, s := range a.Servers {
		stores[i] = s.Store
	}
	a.Badge(badge(stores))
}

func (a *App) notificationBody(s *Server, p *store.Post) string {
	name := a.UserName(s, p.UserID)
	msg := p.Message
	if msg == "" && len(p.Files) > 0 {
		msg = "sent a file"
	}
	return name + ": " + msg
}

// AskUser loads a user again, for their profile.
func (a *App) AskUser(s *Server, id store.UserID) {
	Call(a, s, func(ctx context.Context, c *mm.Client) ([]store.User, error) { return c.Users(ctx, []store.UserID{id}) }, func(us []store.User) {
		a.apply(s, store.UsersLoaded{Users: us})
	})
}

// askUsers fetches users not loaded yet, and the statuses of all of them.
func (a *App) askUsers(s *Server, ids []store.UserID) {
	c := s.Client()
	if c == nil {
		return
	}
	if s.asked == nil {
		s.asked = map[store.UserID]bool{}
	}
	var missing []store.UserID
	for _, id := range ids {
		if id == "" || s.asked[id] {
			continue
		}
		if _, ok := s.Store.Users[id]; ok {
			continue
		}
		s.asked[id] = true
		missing = append(missing, id)
	}
	if len(missing) == 0 {
		return
	}
	st := s.Store
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		users, err := c.Users(ctx, missing)
		statuses, _ := c.Statuses(ctx, missing)
		a.Run(func() {
			if s.Store != st {
				return
			}
			if err != nil {
				for _, id := range missing {
					delete(s.asked, id)
				}
				return
			}
			s.Store.Apply(store.UsersLoaded{Users: users})
			if statuses != nil {
				s.Store.Apply(store.StatusesChanged{Statuses: statuses})
			}
		})
	}()
}

func (a *App) loadDMUsers(s *Server) {
	var ids []store.UserID
	for _, ch := range s.Store.Channels {
		if p := ch.DMPartner(s.Store.Me); p != "" {
			ids = append(ids, p)
		}
	}
	a.askUsers(s, ids)
	// Statuses of the DM partners already loaded refresh too.
	c := s.Client()
	if c == nil || len(ids) == 0 {
		return
	}
	st := s.Store
	goRun(a, func() (map[store.UserID]store.Status, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.Statuses(ctx, ids)
	}, func(m map[store.UserID]store.Status, err error) {
		if err == nil && s.Store == st {
			s.Store.Apply(store.StatusesChanged{Statuses: m})
		}
	})
}

func (a *App) restorePlace(s *Server) {
	p := a.last[s.Config.ID]
	if ch := s.Store.Channels[store.ChannelID(p.Channel)]; ch != nil {
		s.Team = store.TeamID(p.Team)
		if _, ok := s.Store.Teams[s.Team]; !ok && len(s.Store.TeamOrder) > 0 {
			s.Team = s.Store.TeamOrder[0]
		}
		a.OpenChannel(s, ch.ID)
		return
	}
	a.openDefault(s)
}

// openDefault opens the team's town square, or its first channel.
func (a *App) openDefault(s *Server) {
	if _, ok := s.Store.Teams[s.Team]; !ok {
		s.Team = ""
		if len(s.Store.TeamOrder) > 0 {
			s.Team = s.Store.TeamOrder[0]
		}
	}
	var first store.ChannelID
	for id, ch := range s.Store.Channels {
		if ch.TeamID != s.Team {
			continue
		}
		if ch.Name == "town-square" {
			a.OpenChannel(s, id)
			return
		}
		if first == "" || id < first {
			first = id
		}
	}
	if first != "" {
		a.OpenChannel(s, first)
	}
}

func (a *App) SelectTeam(s *Server, team store.TeamID) {
	if s.Team == team {
		return
	}
	s.Team = team
	a.openDefault(s)
}

// OpenChannel shows a channel, loading its posts if needed, and marks it read.
func (a *App) OpenChannel(s *Server, id store.ChannelID) {
	a.openChannel(s, id)
	if s.Channel != id {
		return
	}
	if len(s.back) == 0 || s.back[len(s.back)-1] != id {
		s.back = append(s.back, id)
	}
	s.forward = nil
}

func (a *App) openChannel(s *Server, id store.ChannelID) {
	ch := s.Store.Channels[id]
	if ch == nil {
		return
	}
	if ch.TeamID != "" {
		s.Team = ch.TeamID
	}
	s.ThreadsView = false
	if s.Channel != id {
		s.Thread = ""
		s.NewSince = 0
		if m := s.Store.Members[id]; m != nil && s.Store.Unread(id) {
			s.NewSince = m.LastViewedAt
		}
	}
	s.Channel = id
	if t := s.Store.Timelines[id]; t == nil || (len(t.Order) == 0 && !t.Loading) {
		a.fetchPosts(s, id, "")
	}
	a.markViewed(s, id)
	if c := s.Client(); c != nil && ch.Type != store.Direct {
		st := s.Store
		goRun(a, func() (int64, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			return c.MemberCount(ctx, id)
		}, func(n int64, err error) {
			if err == nil && s.Store == st {
				s.Store.Apply(store.ChannelStats{Channel: id, Members: n})
			}
		})
	}
	if a.last == nil {
		a.last = map[config.ServerID]config.Place{}
	}
	a.last[s.Config.ID] = config.Place{Team: string(s.Team), Channel: string(id)}
	a.save()
}

// Back and Forward go through the channels opened, as a browser's history.
func (a *App) Back(s *Server) {
	if len(s.back) < 2 {
		return
	}
	cur := s.back[len(s.back)-1]
	s.back = s.back[:len(s.back)-1]
	s.forward = append(s.forward, cur)
	a.openChannel(s, s.back[len(s.back)-1])
}

func (a *App) Forward(s *Server) {
	if len(s.forward) == 0 {
		return
	}
	next := s.forward[len(s.forward)-1]
	s.forward = s.forward[:len(s.forward)-1]
	s.back = append(s.back, next)
	a.openChannel(s, next)
}

func (a *App) fetchPosts(s *Server, ch store.ChannelID, before store.PostID) {
	c := s.Client()
	if c == nil {
		return
	}
	s.Store.Apply(store.PostsLoading{Channel: ch, Older: before != ""})
	st := s.Store
	goRun(a, func() (store.PostsLoaded, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.Posts(ctx, ch, before)
	}, func(ev store.PostsLoaded, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			s.Store.Apply(store.PostsFailed{Channel: ch})
			a.handleErr(s, err)
			return
		}
		a.apply(s, ev)
	})
}

// LoadOlder fetches the page before the oldest loaded post.
func (a *App) LoadOlder(s *Server, ch store.ChannelID) {
	t := s.Store.Timelines[ch]
	if t == nil || t.Loading || !t.HasOlder || len(t.Order) == 0 {
		return
	}
	a.fetchPosts(s, ch, t.Order[0])
}

func (a *App) handleErr(s *Server, err error) {
	if errors.Is(err, mm.ErrUnauthorized) {
		a.endSession(s, LoggedOut{Err: "Your session expired. Log in again."})
		return
	}
	if !errors.Is(err, context.Canceled) {
		a.showError(err.Error())
	}
}

// markViewed marks a channel read, here and on the server, when it has
// something unread and the user is looking at it.
func (a *App) markViewed(s *Server, id store.ChannelID) {
	c := s.Client()
	if c == nil || !a.focused() || a.Current != s {
		return
	}
	if !s.Store.Unread(id) && s.Store.Mentions(id) == 0 {
		return
	}
	s.Store.Apply(store.ChannelsViewed{Times: map[store.ChannelID]int64{id: time.Now().UnixMilli()}})
	a.updateBadge()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = c.ViewChannel(ctx, id)
	}()
}

// WindowFocused marks the open channel read as the user comes back.
func (a *App) WindowFocused() {
	if s := a.Current; s != nil && s.Channel != "" {
		a.markViewed(s, s.Channel)
	}
}

// Wake reconnects at once after sleep or a network change.
func (a *App) Wake() {
	for _, s := range a.Servers {
		if s.socket != nil {
			s.socket.Wake()
		}
	}
}

// Send posts a message to a channel, or a reply to a thread.
func (a *App) Send(s *Server, ch store.ChannelID, root store.PostID, text string) {
	text = strings.TrimRight(text, " \t\n")
	if strings.TrimSpace(text) == "" || s.Client() == nil {
		return
	}
	if strings.HasPrefix(text, "/") && !strings.HasPrefix(text, "//") {
		a.RunCommand(s, ch, root, text)
		return
	}
	s.seq++
	p := store.Post{ChannelID: ch, UserID: s.Store.Me, RootID: root, Message: text,
		CreateAt:  time.Now().UnixMilli(),
		PendingID: fmt.Sprintf("%s:%d", s.Store.Me, time.Now().UnixMilli()*1000+int64(s.seq%1000))}
	s.Store.Apply(store.PostSending{Post: p})
	a.send(s, p, nil)
}

// SendFiles posts files with an optional message.
func (a *App) SendFiles(s *Server, ch store.ChannelID, root store.PostID, text string, files []store.FileInfo) {
	s.seq++
	p := store.Post{ChannelID: ch, UserID: s.Store.Me, RootID: root, Message: strings.TrimSpace(text), Files: files,
		CreateAt:  time.Now().UnixMilli(),
		PendingID: fmt.Sprintf("%s:%d", s.Store.Me, time.Now().UnixMilli()*1000+int64(s.seq%1000))}
	s.Store.Apply(store.PostSending{Post: p})
	ids := make([]store.FileID, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	a.send(s, p, ids)
}

func (a *App) send(s *Server, p store.Post, files []store.FileID) {
	c := s.Client()
	st := s.Store
	goRun(a, func() (store.Post, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.CreatePost(ctx, mm.NewPost{ChannelID: p.ChannelID, RootID: p.RootID, Message: p.Message, FileIDs: files, PendingID: p.PendingID})
	}, func(created store.Post, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			s.Store.Apply(store.PostFailed{PendingID: p.PendingID})
			if errors.Is(err, mm.ErrUnauthorized) {
				a.handleErr(s, err)
			}
			return
		}
		a.apply(s, store.PostCreated{Post: created})
	})
}

// Retry sends a failed post again.
func (a *App) Retry(s *Server, pending store.PostID) {
	p := s.Store.Posts[pending]
	if p == nil || p.State != store.Failed {
		return
	}
	q := *p
	s.Store.Apply(store.PostSending{Post: q})
	ids := make([]store.FileID, len(q.Files))
	for i, f := range q.Files {
		ids[i] = f.ID
	}
	a.send(s, q, ids)
}

func (a *App) EditPost(s *Server, id store.PostID, text string) {
	c := s.Client()
	if c == nil || strings.TrimSpace(text) == "" {
		return
	}
	st := s.Store
	goRun(a, func() (store.Post, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.EditPost(ctx, id, text)
	}, func(p store.Post, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		a.apply(s, store.PostEdited{Post: p})
	})
}

func (a *App) DeletePost(s *Server, id store.PostID) {
	p := s.Store.Posts[id]
	if p == nil {
		return
	}
	if p.State != store.Sent {
		s.Store.Apply(store.PostDeleted{ID: id, Channel: p.ChannelID})
		return
	}
	c := s.Client()
	ch := p.ChannelID
	st := s.Store
	goRun(a, func() (struct{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return struct{}{}, c.DeletePost(ctx, id)
	}, func(_ struct{}, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		a.apply(s, store.PostDeleted{ID: id, Channel: ch})
		if s.Thread == id {
			s.Thread = ""
		}
	})
}

// ToggleReaction adds the user's reaction to a post, or takes it away.
func (a *App) ToggleReaction(s *Server, id store.PostID, emoji string) {
	p, c := s.Store.Posts[id], s.Client()
	if p == nil || c == nil || p.State != store.Sent {
		return
	}
	me := s.Store.Me
	mine := slices.ContainsFunc(p.Reactions, func(r store.Reaction) bool { return r.UserID == me && r.EmojiName == emoji })
	var ev store.Event = store.ReactionAdded{Post: id, Reaction: store.Reaction{UserID: me, EmojiName: emoji, CreateAt: time.Now().UnixMilli()}}
	if mine {
		ev = store.ReactionRemoved{Post: id, User: me, EmojiName: emoji}
	}
	s.Store.Apply(ev)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if mine {
			err = c.RemoveReaction(ctx, me, id, emoji)
		} else {
			err = c.AddReaction(ctx, me, id, emoji)
		}
		if err != nil {
			a.Run(func() { a.showError("Reaction not saved: " + err.Error()) })
		}
	}()
}

// OpenThread shows a thread in the RHS and loads it.
func (a *App) OpenThread(s *Server, root store.PostID) {
	if p := s.Store.Posts[root]; p != nil && p.RootID != "" {
		root = p.RootID
	}
	s.Thread = root
	a.readThread(s, root)
	a.OpenThreadData(s, root)
}

// OpenThreadData loads a thread, as for a window of its own.
func (a *App) OpenThreadData(s *Server, root store.PostID) {
	c := s.Client()
	if c == nil {
		return
	}
	st := s.Store
	goRun(a, func() (store.ThreadLoaded, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.Thread(ctx, root)
	}, func(ev store.ThreadLoaded, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		a.apply(s, ev)
	})
}

func (a *App) CloseThread(s *Server) { s.Thread = "" }

// LoadChannel loads the posts of a channel when none are, without
// opening it.
func (a *App) LoadChannel(s *Server, id store.ChannelID) {
	if t := s.Store.Timelines[id]; t == nil || len(t.Order) == 0 && !t.Loading {
		a.fetchPosts(s, id, "")
	}
}

// Typing tells the server, at most every few seconds, that the user types.
func (a *App) Typing(s *Server, ch store.ChannelID, root store.PostID) {
	if s.socket == nil || time.Since(s.typedAt) < 3*time.Second {
		return
	}
	s.typedAt = time.Now()
	sock := s.socket
	go sock.Typing(ch, root)
}

// OpenDM opens the DM with a user, creating it on the server if needed.
func (a *App) OpenDM(s *Server, user store.UserID) {
	for id, ch := range s.Store.Channels {
		if ch.Type == store.Direct && ch.DMPartner(s.Store.Me) == user {
			a.OpenChannel(s, id)
			return
		}
	}
	c := s.Client()
	if c == nil {
		return
	}
	me, st := s.Store.Me, s.Store
	goRun(a, func() (store.ChannelUpserted, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ch, err := c.DirectChannel(ctx, me, user)
		if err != nil {
			return store.ChannelUpserted{}, err
		}
		m, err := c.Member(ctx, ch.ID)
		return store.ChannelUpserted{Channel: ch, Member: &m}, err
	}, func(ev store.ChannelUpserted, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		a.apply(s, ev)
		a.OpenChannel(s, ev.Channel.ID)
	})
}

// SearchUsers finds users for autocomplete and the switcher.
func (a *App) SearchUsers(s *Server, term string, done func([]store.User)) {
	c := s.Client()
	if c == nil {
		return
	}
	st := s.Store
	goRun(a, func() ([]store.User, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return c.SearchUsers(ctx, term)
	}, func(users []store.User, err error) {
		if err != nil || s.Store != st {
			return
		}
		s.Store.Apply(store.UsersLoaded{Users: users})
		done(users)
	})
}

// UserName is how a user shows: their username, as the webapp's default.
func (a *App) UserName(s *Server, id store.UserID) string {
	if u := s.Store.Users[id]; u != nil {
		switch s.Store.Pref("display_settings", "name_format") {
		case "full_name":
			if n := u.FullName(); n != "" {
				return n
			}
		case "nickname_full_name":
			if u.Nickname != "" {
				return u.Nickname
			}
			if n := u.FullName(); n != "" {
				return n
			}
		}
		return u.Username
	}
	return "Someone"
}

// ChannelName is how a channel shows in the sidebar and header.
func (a *App) ChannelName(s *Server, ch *store.Channel) string {
	switch ch.Type {
	case store.Direct:
		p := ch.DMPartner(s.Store.Me)
		name := a.UserName(s, p)
		if p == s.Store.Me {
			name += " (you)"
		}
		return name
	case store.Group:
		if ch.DisplayName != "" {
			return ch.DisplayName
		}
	}
	if ch.DisplayName != "" {
		return ch.DisplayName
	}
	return ch.Name
}
