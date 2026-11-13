package app

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// Permalink is the web address of a post, as the webapp copies it.
func (a *App) Permalink(s *Server, id store.PostID) string {
	team := ""
	if t := s.Store.Teams[s.Team]; t != nil {
		team = t.Name
	}
	return s.URL.String() + "/" + team + "/pl/" + string(id)
}

// OpenURL opens a web link in the default browser.
func (a *App) OpenURL(link string) {
	if a.OpenExternal != nil {
		a.OpenExternal(link)
	}
}

// OpenPermalink opens a link to a post or channel of a server the user
// added, in the app. It reports whether the link was one.
func (a *App) OpenPermalink(from *Server, link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	for _, s := range a.Servers {
		if u.Host != s.URL.Host || !strings.HasPrefix(u.Path, s.URL.Path+"/") {
			continue
		}
		if _, in := s.Session.(LoggedIn); !in {
			return false
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(u.Path, s.URL.Path), "/"), "/")
		if len(parts) < 3 {
			return false
		}
		team, kind, name := parts[0], parts[1], parts[2]
		a.SelectServer(s)
		switch kind {
		case "pl":
			a.OpenPost(s, store.PostID(name))
			return true
		case "channels":
			for id, ch := range s.Store.Channels {
				if ch.Name == name && (ch.TeamID == "" || s.Store.Teams[ch.TeamID] != nil && s.Store.Teams[ch.TeamID].Name == team) {
					a.OpenChannel(s, id)
					return true
				}
			}
		case "messages":
			if strings.HasPrefix(name, "@") {
				for id, usr := range s.Store.Users {
					if usr.Username == name[1:] {
						a.OpenDM(s, id)
						return true
					}
				}
			}
		}
		return false
	}
	return false
}

// OpenDeepLink opens a mattermost:// link: the server, and the channel
// or post it names.
func (a *App) OpenDeepLink(link string) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return
	}
	for _, scheme := range []string{"https", "http"} {
		web := *u
		web.Scheme = scheme
		if a.OpenPermalink(a.Current, web.String()) {
			return
		}
	}
	for _, s := range a.Servers {
		if s.URL.Host == u.Host {
			a.SelectServer(s)
			return
		}
	}
}

// OpenPost opens the channel of a post, and its thread.
func (a *App) OpenPost(s *Server, id store.PostID) {
	if p := s.Store.Posts[id]; p != nil {
		a.OpenChannel(s, p.ChannelID)
		a.OpenThread(s, id)
		return
	}
	c := s.Client()
	if c == nil {
		return
	}
	st := s.Store
	goRun(a, func() (store.ThreadLoaded, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.Thread(ctx, id)
	}, func(ev store.ThreadLoaded, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		for _, p := range ev.Posts {
			if p.ID == id {
				a.OpenChannel(s, p.ChannelID)
				s.Thread = ev.Root
				a.apply(s, ev)
				return
			}
		}
	})
}

// Download saves a file to the Downloads folder and shows it in Finder.
func (a *App) Download(s *Server, f store.FileInfo) {
	c := s.Client()
	if c == nil {
		return
	}
	goRun(a, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		data, err := c.File(ctx, f.ID, mm.FileFull)
		if err != nil {
			return "", err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path := freeName(filepath.Join(home, "Downloads"), filepath.Base(f.Name))
		return path, os.WriteFile(path, data, 0o644)
	}, func(path string, err error) {
		if err != nil {
			a.showError("Download failed: " + err.Error())
			return
		}
		if a.Reveal != nil {
			a.Reveal(path)
		}
	})
}

// FetchFile saves a file of a post in a folder of the app, once, and
// passes its path to done, for previewing or opening it.
func (a *App) FetchFile(s *Server, f store.FileInfo, done func(path string)) {
	c := s.Client()
	if c == nil {
		return
	}
	goRun(a, func() (string, error) {
		dir := filepath.Join(os.TempDir(), "mattermost-go", string(f.ID))
		path := filepath.Join(dir, filepath.Base(f.Name))
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		data, err := c.File(ctx, f.ID, mm.FileFull)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		return path, os.WriteFile(path, data, 0o600)
	}, func(path string, err error) {
		if err != nil {
			a.showError("Opening " + f.Name + ": " + err.Error())
			return
		}
		done(path)
	})
}

// QuickLookFile previews a file in the Quick Look panel, or opens it in
// its app where Quick Look is missing.
func (a *App) QuickLookFile(s *Server, f store.FileInfo) {
	a.FetchFile(s, f, func(path string) {
		if a.QuickLook != nil && a.QuickLook(path) == nil {
			return
		}
		if a.OpenPath != nil {
			_ = a.OpenPath(path)
		}
	})
}

// OpenFile opens a file in its app.
func (a *App) OpenFile(s *Server, f store.FileInfo) {
	a.FetchFile(s, f, func(path string) {
		if a.OpenPath != nil {
			_ = a.OpenPath(path)
		}
	})
}

// freeName returns a path in dir for name that no file has yet.
func freeName(dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	path := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(dir, base+" ("+strconv.Itoa(i)+")"+ext)
	}
}

// Upload sends files to a channel and reports their infos for a post.
func (a *App) Upload(s *Server, ch store.ChannelID, names []string, data [][]byte, done func([]store.FileInfo)) {
	c := s.Client()
	if c == nil {
		return
	}
	st := s.Store
	goRun(a, func() ([]store.FileInfo, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		var out []store.FileInfo
		for i := range names {
			f, err := c.UploadFile(ctx, ch, names[i], data[i])
			if err != nil {
				return nil, err
			}
			out = append(out, f)
		}
		return out, nil
	}, func(files []store.FileInfo, err error) {
		if s.Store != st {
			return
		}
		if err != nil {
			a.handleErr(s, err)
			return
		}
		done(files)
	})
}
