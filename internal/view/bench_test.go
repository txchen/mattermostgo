package view

import (
	"fmt"
	"testing"
	"time"

	"github.com/txchen/mattermostgo/internal/store"
)

// bigMessages are the shapes of the posts of the big channel, as
// scripts/seed -big writes them.
var bigMessages = []string{
	"Post %d: a plain line of text, as most posts are.",
	"Post %d with **bold**, _italic_, `code` and a [link](https://mattermost.com) :tada:",
	"Post %d asks @alice and @bob to look at ~dev :eyes:",
	"Post %d has a list:\n\n- one\n- two\n- three",
	"Post %d has code:\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
	"Post %d is longer. Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
	"Post %d 中文消息，混合 English 和 emoji :rocket:",
	"| post | %d |\n|---|---|\n| a | b |",
}

// Scrolling a channel of 5,000 loaded posts: the time of a frame that
// scrolls by a screen, rows built and laid out for the first time
// included once the list has gone round.
func BenchmarkScrollBigChannel(b *testing.B) {
	h := newHarness(&testing.T{})
	posts := make([]store.Post, 5000)
	start := time.Now().Add(-48 * time.Hour).UnixMilli()
	users := []store.UserID{"bob", "me", "bob"}
	for i := range posts {
		posts[i] = store.Post{ID: store.PostID(fmt.Sprintf("big%05d", i)), ChannelID: "town", UserID: users[i%3],
			CreateAt: start + int64(i)*30_000, Message: fmt.Sprintf(bigMessages[i%len(bigMessages)], i+1)}
	}
	h.srv.Store.Apply(store.PostsLoaded{Channel: "town", Posts: posts})
	h.tt.Frame()
	b.ResetTimer()
	dy := float32(-600)
	for i := range b.N {
		if i%400 == 0 {
			dy = -dy
		}
		h.tt.Scroll(700, 300, 0, dy)
	}
}
