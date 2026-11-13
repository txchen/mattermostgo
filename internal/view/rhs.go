package view

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/mm"
	"github.com/txchen/mattermostgo/internal/store"
)

// panel is what the RHS shows besides a thread: search results, a list of
// posts, or a channel's info. A thread opened from it shows over it, with
// a way back.
type panel interface{ isPanel() }

type postsKind int

const (
	searchResults postsKind = iota
	recentMentions
	savedPosts
	pinnedPosts
)

// postsPanel lists posts the server found: search results, mentions,
// saved posts, or a channel's pinned posts.
type postsPanel struct {
	kind    postsKind
	query   string
	channel store.ChannelID
	posts   []store.PostID
	files   []mm.FileResult
	// showFiles shows the files found instead of the posts.
	showFiles           bool
	loaded, filesLoaded bool
}

// infoPanel shows a channel: its header, purpose, bookmarks, members and
// files.
type infoPanel struct {
	channel   store.ChannelID
	tab       int
	members   []mm.MemberInfo
	files     []mm.FileResult
	bookmarks []mm.Bookmark
	// asked marks what has been asked of the server, and loaded what came.
	asked, loaded map[int]bool
	// noBookmarks is set when the server has no bookmarks, as without a
	// license for them.
	noBookmarks bool
	bookmark    string
	bookmarkURL string
}

func (*postsPanel) isPanel() {}
func (*infoPanel) isPanel()  {}

// openPanel shows a panel in the RHS, closing the thread there.
func (v *View) openPanel(s *app.Server, p panel) {
	v.panel, v.panelServer = p, s
	s.Thread = ""
}

func (v *View) currentPanel(s *app.Server) panel {
	if v.panelServer != s {
		return nil
	}
	return v.panel
}

// search runs a search from the top bar.
func (v *View) search(s *app.Server, terms string) {
	terms = strings.TrimSpace(terms)
	if terms == "" {
		return
	}
	p := &postsPanel{kind: searchResults, query: terms}
	v.openPanel(s, p)
	team := s.Team
	v.App.ListPosts(s, func(ctx context.Context, c *mm.Client) (mm.PostResults, error) { return c.Search(ctx, team, terms) },
		func(ids []store.PostID) { p.posts, p.loaded = ids, true })
	app.Call(v.App, s, func(ctx context.Context, c *mm.Client) ([]mm.FileResult, error) {
		return c.SearchFiles(ctx, team, terms)
	},
		func(fs []mm.FileResult) { p.files, p.filesLoaded = fs, true })
}

func (v *View) openMentions(s *app.Server) {
	me := s.Me()
	if me == nil {
		return
	}
	p := &postsPanel{kind: recentMentions}
	v.openPanel(s, p)
	team, terms := s.Team, "@"+me.Username
	v.App.ListPosts(s, func(ctx context.Context, c *mm.Client) (mm.PostResults, error) { return c.Search(ctx, team, terms) },
		func(ids []store.PostID) { p.posts, p.loaded = ids, true })
}

func (v *View) openSaved(s *app.Server) {
	p := &postsPanel{kind: savedPosts}
	v.openPanel(s, p)
	me := s.Store.Me
	v.App.ListPosts(s, func(ctx context.Context, c *mm.Client) (mm.PostResults, error) { return c.Saved(ctx, me) },
		func(ids []store.PostID) { p.posts, p.loaded = ids, true })
}

func (v *View) openPinned(s *app.Server, ch store.ChannelID) {
	p := &postsPanel{kind: pinnedPosts, channel: ch}
	v.openPanel(s, p)
	v.App.ListPosts(s, func(ctx context.Context, c *mm.Client) (mm.PostResults, error) { return c.Pinned(ctx, ch) },
		func(ids []store.PostID) { p.posts, p.loaded = ids, true })
}

func (v *View) openInfo(s *app.Server, ch store.ChannelID, tab int) {
	v.openPanel(s, &infoPanel{channel: ch, tab: tab, asked: map[int]bool{}, loaded: map[int]bool{}})
}

// rhsHeader is the top of the RHS, with a way back and a close button.
func (v *View) rhsHeader(c *ui.Context, title, detail string, back, close func()) {
	t := c.Theme()
	ui.Row(c).Height(62).Padding(0, 12, 0, 16).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		if back != nil && iconButton(c, "chevron.left", "Back", 13, t.TextMuted, t.SurfaceHover).Clicked() {
			back()
		}
		ui.Text(c, title).FontWeight(700).FontSize(16).MaxLines(1).Shrink(0)
		if detail != "" {
			ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted).MaxLines(1).Shrink(1)
		}
		ui.Spacer(c)
		if iconButton(c, "xmark", "Close", 14, t.TextMuted, t.SurfaceHover).Clicked() {
			close()
		}
	})
}

// rhs is the right-hand side: a thread, or else the open panel.
func (v *View) rhs(c *ui.Context, s *app.Server) {
	if s.Thread != "" {
		v.thread(c, s)
		return
	}
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Background(t.Background).Children(func() {
		switch p := v.currentPanel(s).(type) {
		case *postsPanel:
			v.postsPanel(c, s, p)
		case *infoPanel:
			v.infoPanel(c, s, p)
		}
	})
}

func (v *View) closePanel() { v.panel, v.panelServer = nil, nil }

func (v *View) postsPanel(c *ui.Context, s *app.Server, p *postsPanel) {
	t := c.Theme()
	title, detail, empty := "", "", ""
	switch p.kind {
	case searchResults:
		title, detail, empty = "Search Results", p.query, "No results for "+p.query
	case recentMentions:
		title, empty = "Recent Mentions", "No mentions yet"
	case savedPosts:
		title, empty = "Saved messages", "No saved messages yet. Save a message from its menu to find it here."
	case pinnedPosts:
		title, empty = "Pinned messages", "No pinned messages yet"
		if ch := s.Store.Channels[p.channel]; ch != nil {
			detail = v.App.ChannelName(s, ch)
		}
	}
	v.rhsHeader(c, title, detail, nil, v.closePanel)
	if p.kind == searchResults {
		tab := 0
		if p.showFiles {
			tab = 1
		}
		ui.Segmented(c.Key("results"), &tab, fmt.Sprintf("Messages %d", len(p.posts)), fmt.Sprintf("Files %d", len(p.files))).Margin(8, 16)
		p.showFiles = tab == 1
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(4, 0, 16, 0).Children(func() {
			if p.showFiles {
				if !p.filesLoaded {
					ui.Row(c).Center().Padding(24).Children(func() { ui.Spinner(c) })
				} else if len(p.files) == 0 {
					ui.Text(c, "No files for "+p.query).TextColor(t.TextMuted).Padding(24, 16)
				}
				for _, f := range p.files {
					v.fileResult(c.Key(f.File.ID), s, f)
				}
				return
			}
			if !p.loaded {
				ui.Row(c).Center().Padding(24).Children(func() { ui.Spinner(c) })
				return
			}
			shown := 0
			for _, id := range p.posts {
				post := s.Store.Posts[id]
				if post == nil || p.kind == savedPosts && !s.Store.Saved(id) || p.kind == pinnedPosts && !post.Pinned {
					continue
				}
				shown++
				v.resultCard(c.Key(id), s, post)
			}
			if shown == 0 {
				ui.Text(c, empty).TextColor(t.TextMuted).Padding(24, 16)
			}
		})
	})
}

// resultCard shows a post found, with its channel, as the webapp's
// search results: a click opens its thread, Jump its channel.
func (v *View) resultCard(c *ui.Context, s *app.Server, post *store.Post) {
	t := c.Theme()
	p := colors(c)
	a := v.App
	card := ui.Column(c).Padding(10, 16).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ch := s.Store.Channels[post.ChannelID]; ch != nil {
				ui.Text(c, a.ChannelName(s, ch)).FontSize(12).FontWeight(600).TextColor(t.TextMuted).MaxLines(1).Shrink(1)
			}
			ui.Spacer(c)
			if ui.Link(c, "Jump", "").FontSize(12).Clicked() {
				a.OpenPost(s, post.ID)
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
			v.userButton(c, s, post, 32)
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
					ui.Text(c, a.UserName(s, post.UserID)).FontWeight(700).FontSize(14)
					ts := time.UnixMilli(post.CreateAt)
					ui.Text(c, ts.Format("Jan 2")+" "+v.clock(ts)).FontSize(12).TextColor(t.TextMuted)
				})
				root := post.ID
				if post.RootID != "" {
					root = post.RootID
				}
				v.postBody(c, s, post, root)
				if n := post.ReplyCount; n > 0 {
					label := fmt.Sprint(n, " replies")
					if n == 1 {
						label = "1 reply"
					}
					if ui.Link(c, label, "").FontSize(12).TextColor(p.link).Clicked() {
						a.OpenThread(s, root)
					}
				}
			})
		})
	})
}

func (v *View) fileResult(c *ui.Context, s *app.Server, f mm.FileResult) {
	t := c.Theme()
	row := ui.ButtonBase(c).FillWidth().Padding(8, 16).Label(f.File.Name)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fileGlyph(f.File)).FontSize(22)
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				ui.Text(c, f.File.Name).FontWeight(600).MaxLines(1)
				detail := size(f.File.Size)
				if ch := s.Store.Channels[f.Channel]; ch != nil {
					detail += " · " + v.App.ChannelName(s, ch)
				}
				detail += " · " + time.UnixMilli(f.CreateAt).Format("Jan 2")
				ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
			})
		})
	})
	if row.Clicked() {
		v.preview(s, []store.FileInfo{f.File}, 0)
	}
}

func (v *View) infoPanel(c *ui.Context, s *app.Server, p *infoPanel) {
	t := c.Theme()
	a := v.App
	ch := s.Store.Channels[p.channel]
	if ch == nil {
		v.closePanel()
		return
	}
	v.rhsHeader(c, "Info", a.ChannelName(s, ch), nil, v.closePanel)
	ui.Segmented(c.Key("tabs"), &p.tab, "About", "Members", "Files").Margin(8, 16)
	if !p.asked[p.tab] {
		p.asked[p.tab] = true
		switch p.tab {
		case 0:
			app.Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.Bookmark, error) {
				bs, err := c.Bookmarks(ctx, p.channel)
				if err != nil {
					// Servers without the license answer with an error;
					// the channel then shows no bookmarks.
					return nil, nil
				}
				return bs, err
			}, func(bs []mm.Bookmark) { p.bookmarks, p.noBookmarks, p.loaded[0] = bs, bs == nil, true })
		case 1:
			a.Members(s, p.channel, func(ms []mm.MemberInfo) { p.members, p.loaded[1] = ms, true })
		case 2:
			team, name := s.Team, ch.Name
			app.Call(a, s, func(ctx context.Context, c *mm.Client) ([]mm.FileResult, error) {
				return c.SearchFiles(ctx, team, "in:"+name)
			},
				func(fs []mm.FileResult) { p.files, p.loaded[2] = fs, true })
		}
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(8, 16, 16, 16).Gap(10).Children(func() {
			switch p.tab {
			case 0:
				section := func(label, text string, edit func()) {
					ui.Column(c).Gap(4).Children(func() {
						ui.Row(c).Children(func() {
							ui.Text(c, strings.ToUpper(label)).FontSize(11).FontWeight(600).TextColor(t.TextMuted)
							ui.Spacer(c)
							if edit != nil && ui.Link(c, "Edit", "").FontSize(12).Label("Edit "+label).Clicked() {
								edit()
							}
						})
						if text == "" {
							text = "None"
						}
						ui.Text(c, text).Selectable()
					})
				}
				if ch.Type == store.Open || ch.Type == store.Private {
					section("Purpose", ch.Purpose, func() {
						v.dialog = &textDialog{title: "Edit Purpose", label: "Purpose", value: ch.Purpose, submit: "Save", lines: true,
							save: func(text string) { a.PatchChannel(s, ch.ID, mm.ChannelPatch{Purpose: &text}, nil) }}
					})
				}
				section("Header", ch.Header, func() {
					v.dialog = &textDialog{title: "Edit Header", label: "Header", value: ch.Header, submit: "Save", lines: true,
						save: func(text string) { a.PatchChannel(s, ch.ID, mm.ChannelPatch{Header: &text}, nil) }}
				})
				if ui.Link(c, "Pinned messages", "").Clicked() {
					v.openPinned(s, ch.ID)
				}
				if !p.noBookmarks && p.loaded[0] {
					v.bookmarks(c, s, p)
				}
			case 1:
				if !p.loaded[1] {
					ui.Spinner(c)
					return
				}
				if ui.Button(c, "Add Members").Clicked() {
					v.openAddMembers(s, ch.ID)
				}
				for _, m := range p.members {
					ui.Row(c.Key(m.User)).Gap(10).AlignItems(ui.Center).Children(func() {
						v.avatar(c, s, m.User, 28)
						ui.Text(c, a.UserName(s, m.User)).Grow(1).MaxLines(1)
						if m.Admin {
							ui.Text(c, "Admin").FontSize(12).TextColor(t.TextMuted)
						}
					})
				}
			case 2:
				if !p.loaded[2] {
					ui.Spinner(c)
					return
				}
				if len(p.files) == 0 {
					ui.Text(c, "No files yet").TextColor(t.TextMuted)
				}
				for _, f := range p.files {
					v.fileResult(c.Key(f.File.ID), s, f)
				}
			}
		})
	})
}

func (v *View) bookmarks(c *ui.Context, s *app.Server, p *infoPanel) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "BOOKMARKS").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
		for _, b := range p.bookmarks {
			ui.Row(c.Key(b.ID)).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.Link(c, b.DisplayName, "").Tooltip(b.LinkURL).Clicked() {
					v.openLink(s, b.LinkURL)
				}
				ui.Spacer(c)
				if iconButton(c, "xmark", "Remove "+b.DisplayName, 10, t.TextMuted, t.SurfaceHover).Clicked() {
					id, ch := b.ID, p.channel
					app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (struct{}, error) {
						return struct{}{}, c.DeleteBookmark(ctx, ch, id)
					},
						func(struct{}) {
							p.bookmarks = slices.DeleteFunc(p.bookmarks, func(x mm.Bookmark) bool { return x.ID == id })
						})
				}
			})
		}
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c.Key("bm-name"), &p.bookmark).Placeholder("Title").Label("Bookmark title").Width(110)
			ui.TextInput(c.Key("bm-url"), &p.bookmarkURL).Placeholder("https://…").Label("Bookmark link").Grow(1)
			if ui.Button(c, "Add").Disabled(p.bookmark == "" || p.bookmarkURL == "").Clicked() {
				b, ch := mm.Bookmark{DisplayName: p.bookmark, LinkURL: p.bookmarkURL}, p.channel
				app.Call(v.App, s, func(ctx context.Context, c *mm.Client) (mm.Bookmark, error) { return c.AddBookmark(ctx, ch, b) },
					func(nb mm.Bookmark) { p.bookmarks, p.bookmark, p.bookmarkURL = append(p.bookmarks, nb), "", "" })
			}
		})
	})
}
