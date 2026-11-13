package markdown

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	mdview "github.com/egoist/mygo/ui/markdown"
)

func TestPrepare(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hi @alice.", "hi [@alice](mm-mention:alice)."},
		{"@first.last thanks", "[@first.last](mm-mention:first.last) thanks"},
		{"see ~town-square", "see [~town-square](mm-channel:town-square)"},
		{"@user_name", `[@user\_name](mm-mention:user_name)`},
		{"mail me at bob@example.com", "mail me at bob@example.com"},
		{"great :+1: :tada:", "great 👍 🎉"},
		{"not an emoji :nope: here", "not an emoji :nope: here"},
		{"time 10:30:00", "time 10:30:00"},
		{"`@alice :+1:` and @bob", "`@alice :+1:` and [@bob](mm-mention:bob)"},
		{"```\n@alice :+1:\n```\n@bob", "```\n@alice :+1:\n```\n[@bob](mm-mention:bob)"},
		{"    @indented code", "    @indented code"},
		{"[@alice](https://x.y/@alice) https://x.y/@bob", "[@alice](https://x.y/@alice) https://x.y/@bob"},
		{`\@alice`, `\@alice`},
		{"~~strike~~", "~~strike~~"},
	}
	for _, tc := range cases {
		if got := Prepare(tc.in); got != tc.want {
			t.Errorf("Prepare(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

// Generated messages keep their code exactly, and their text outside
// mentions, channels and emoji.
func TestPrepareKeepsCode(t *testing.T) {
	parts := []string{"@a", "@b.c", "~d", ":smile:", ":x", "word", " ", "\n", "`", "``", "```\n", "\\", "[l](u)", "<http://h>", "http://h/@z", "_", "*", "."}
	for seed := range uint64(2000) {
		r := rand.New(rand.NewPCG(seed, 1))
		var b strings.Builder
		for range 3 + r.IntN(20) {
			b.WriteString(parts[r.IntN(len(parts))])
		}
		code := "```\n" + b.String() + "\n```\n"
		in := b.String() + "\n" + code
		got := Prepare(in)
		if !strings.HasSuffix(got, code) && !strings.Contains(b.String(), "```") {
			t.Fatalf("seed %d: code block changed:\n%q\n%q", seed, in, got)
		}
		if !strings.ContainsAny(in, ":@~") && got != in {
			t.Fatalf("seed %d: changed %q to %q", seed, in, got)
		}
	}
}

// A mention shows as a link that the client's OnLink receives.
func TestMentionLinkClicks(t *testing.T) {
	var opened []string
	o := &mdview.Options{OnLink: func(u string) { opened = append(opened, u) }}
	src := Prepare("ping @alice in ~general :rocket:")
	tt := ui.NewTester(func(c *ui.Context) { mdview.View(c, src, o) }, 500, 200)
	if err := tt.Click("@alice"); err != nil {
		t.Fatalf("%v in %q", err, tt.Texts())
	}
	if err := tt.Click("~general"); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 2 || opened[0] != "mm-mention:alice" || opened[1] != "mm-channel:general" {
		t.Errorf("opened %q", opened)
	}
	if !strings.Contains(strings.Join(tt.Texts(), " "), "🚀") {
		t.Errorf("no emoji in %q", tt.Texts())
	}
}
