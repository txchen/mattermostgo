// Mattermost Go is a native desktop client for Mattermost.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/media"
	"github.com/txchen/mattermostgo/internal/view"
)

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	// Linux and Windows update themselves from the GitHub releases; the
	// Mac app updates with Homebrew.
	if runtime.GOOS != "darwin" {
		mygo.Use(native.Plugin)
	}
	var win *mygo.Window
	// popouts are the windows of channels and threads opened on their
	// own, which draw again with the main window.
	var popouts []*mygo.Window
	a := &app.App{
		HC:      &http.Client{Transport: netproxy.Transport()},
		Secrets: mygo.Secrets,
	}
	// Until the window exists, nothing draws: work arriving early runs on
	// the main thread alone.
	a.Run = func(fn func()) {
		if win != nil {
			win.Update(fn)
			for _, w := range popouts {
				w.Invalidate()
			}
		} else {
			mygo.RunOnMain(fn)
		}
	}
	var v *view.View
	var deniedOnce sync.Once
	mygo.App.OnOpenURL(func(url string) {
		a.Run(func() { a.OpenDeepLink(url) })
	})
	mygo.App.WhenReady(func() {
		data, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Fatal(err)
		}
		cache, _ := mygo.App.Path(mygo.PathCache)
		a.ConfigPath = filepath.Join(data, "servers.json")
		v = view.New(a, media.New(a.Run, filepath.Join(cache, "images"), 96<<20))
		v.Hooks = view.Hooks{
			PickFiles: func(done func([]string)) {
				go func() {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Attach Files", Multiple: true})
					if err == nil && len(paths) > 0 {
						a.Run(func() { done(paths) })
					}
				}()
			},
			ClipboardImage: mygo.Clipboard.ReadImage,
			OpenWindow: func(title string, build func(c *ui.Context)) {
				w := mygo.NewWindow(mygo.WindowOptions{
					Title: title, Width: 720, Height: 760, MinWidth: 420, MinHeight: 360,
					TitleBarStyle: mygo.TitleBarHidden, Content: ui.View(build),
				})
				popouts = append(popouts, w)
				w.OnClosed(func() {
					for i, x := range popouts {
						if x == w {
							popouts = append(popouts[:i], popouts[i+1:]...)
							break
						}
					}
				})
				w.Show()
			},
		}
		if runtime.GOOS != "darwin" {
			v.Hooks.CheckForUpdates = updater.CheckForUpdates
		}
		a.Error = func(msg string) {
			v.ShowError(msg)
			if win != nil {
				win.Invalidate()
			}
		}
		a.OpenExternal = func(url string) { _ = mygo.Shell.OpenExternal(url) }
		a.Reveal = mygo.Shell.ShowItemInFolder
		a.QuickLook = mygo.Shell.PreviewItem
		a.OpenPath = mygo.Shell.OpenPath
		a.Badge = func(mentions int64, unread bool) {
			switch {
			case mentions > 0:
				mygo.App.Dock.SetBadge(fmt.Sprint(mentions))
			case unread:
				mygo.App.Dock.SetBadge("•")
			default:
				mygo.App.Dock.SetBadge("")
			}
		}
		a.Notify = func(n app.Notification) {
			note := mygo.NewNotification(mygo.NotificationOptions{Title: n.Title, Body: n.Body, Silent: n.Silent, Group: string(n.Server) + "/" + string(n.Channel)})
			note.OnClick(func() {
				a.Run(func() {
					for _, s := range a.Servers {
						if s.Config.ID == n.Server {
							a.SelectServer(s)
							a.OpenChannel(s, n.Channel)
						}
					}
					win.Show()
					win.Focus()
				})
			})
			go func() {
				// Show waits for the user the first time, as macOS asks
				// whether the app may notify.
				err := note.Show()
				if err != nil || os.Getenv("MMGO_DEBUG") != "" {
					log.Printf("notification %q: %v", n.Title, err)
				}
				if errors.Is(err, mygo.ErrNotificationsDenied) {
					deniedOnce.Do(func() {
						a.Run(func() {
							a.Error("Notifications are off for Mattermost Go. Turn them on in System Settings › Notifications.")
						})
					})
				}
			}()
		}
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Mattermost Go",
			Width:         1200,
			Height:        800,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(v.Build),
		})
		a.Focused = func() bool { return win.IsFocused() && !mygo.App.IsHidden() }
		win.OnFocus(func() { a.Run(a.WindowFocused) })
		if err := a.Start(); err != nil {
			a.Error("Reading the servers: " + err.Error())
		}
		mygo.Power.OnResume(func() { a.Run(a.Wake) })
		mygo.Network.OnChanged(func(mygo.NetworkStatus) {
			// Let the new network settle before reconnecting.
			time.AfterFunc(2*time.Second, func() { a.Run(a.Wake) })
		})
		mygo.App.OnSecondInstance(func([]string, string) {
			a.Run(func() {
				win.Show()
				win.Focus()
			})
		})
		mygo.App.OnActivate(func(visible bool) {
			if !visible {
				win.Show()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
