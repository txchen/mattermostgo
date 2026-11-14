package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/txchen/mattermostgo/internal/app"
	"github.com/txchen/mattermostgo/internal/store"
)

// A user at the computer stays online however long the app runs, and goes
// away once they stop using it. The server makes away a user it hears
// nothing from for its away timeout, at the next ping of the WebSocket;
// the test shortens the timeout to 10 s, and waits for a ping or two, so
// it runs only with MM_E2E_SLOW set.
func TestAwayFollowsActivity(t *testing.T) {
	base := serverURL(t)
	if os.Getenv("MM_E2E_SLOW") == "" {
		t.Skip("MM_E2E_SLOW not set")
	}
	ctx := context.Background()
	carolC, carol := login(t, base, "carol")
	api(t, base, "admin", "PUT", "/config/patch", map[string]any{"TeamSettings": map[string]any{"UserStatusAwayTimeout": 10}})
	t.Cleanup(func() {
		api(t, base, "admin", "PUT", "/config/patch", map[string]any{"TeamSettings": map[string]any{"UserStatusAwayTimeout": 300}})
	})
	app.ActivityEvery = 3 * time.Second
	t.Cleanup(func() { app.ActivityEvery = time.Minute })
	idle := time.Duration(0)
	c := newClient(t)
	c.a.IdleTime = func() time.Duration { return idle }
	s := c.loginAs(base, "carol")
	status := func() store.Status { return must(carolC.Statuses(ctx, []store.UserID{carol.ID}))[carol.ID] }
	c.until("online", func() bool { return status() == store.Online })

	// Active: past two pings of the server, carol is still online.
	deadline := time.Now().Add(130 * time.Second)
	c.untilFor("two pings active", 140*time.Second, func() bool {
		if st := status(); st != store.Online {
			t.Fatalf("active carol went %s", st)
		}
		return time.Now().After(deadline)
	})

	// Idle: she goes away.
	idle = 10 * time.Minute
	c.untilFor("away when idle", 90*time.Second, func() bool { return status() == store.Away })
	_ = s
}
