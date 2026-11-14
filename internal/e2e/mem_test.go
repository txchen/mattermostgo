package e2e

import (
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// TestMemoryOpening logs in, opens Big Channel and draws it, then writes
// the heap to MM_HEAP and logs its size, to compare builds:
//
//	MM_E2E_URL=http://localhost:8065 MM_HEAP=/tmp/new.heap go test ./internal/e2e -run TestMemoryOpening -v
func TestMemoryOpening(t *testing.T) {
	base := serverURL(t)
	out := os.Getenv("MM_HEAP")
	if out == "" {
		t.Skip("MM_HEAP not set")
	}
	c := newClient(t)
	c.a.AddServer("Local", base, func(error) {})
	c.until("added", func() bool { return len(c.a.Servers) == 1 })
	c.a.Login(c.a.Current, "alice", "Password1!", "")
	c.until("ready", func() bool { return c.a.Current.Ready })
	s := c.a.Current
	big := channelByName(s, "big")
	c.a.OpenChannel(s, big)
	c.until("big loaded", func() bool { tl := s.Store.Timelines[big]; return tl != nil && !tl.Loading && len(tl.Order) > 0 })
	settled := time.Now().Add(3 * time.Second)
	c.until("settled", func() bool { return time.Now().After(settled) })
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("heap in use %.1f MB, objects %d, sys %.1f MB", float64(m.HeapInuse)/(1<<20), m.HeapObjects, float64(m.Sys)/(1<<20))
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pprof.WriteHeapProfile(f)
}
