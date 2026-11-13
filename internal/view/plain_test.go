package view

import (
	"html"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Messages that skip the Markdown view are ones the Markdown parser reads
// as one paragraph of the same text, for generated messages.
func TestPlainTextIsOneParagraph(t *testing.T) {
	alphabet := []string{"a", "b", "Z", "1", "9", " ", " ", "\n", ".", ",", ":", ";", "!", "?", "'", "\"", "(", ")", "-", "+", "=", "/", "中", "文", "%", "$", "^", "{", "}"}
	plain := 0
	for seed := range uint64(20000) {
		r := rand.New(rand.NewPCG(seed, 3))
		var b strings.Builder
		for range 1 + r.IntN(30) {
			b.WriteString(alphabet[r.IntN(len(alphabet))])
		}
		msg := b.String()
		if !plainText(msg) {
			continue
		}
		plain++
		d := parser.New(parser.GFM) // as markdownOptions reads posts
		d.Set(msg)
		got := strings.ReplaceAll(d.HTML(), "<br />", "")
		var lines []string
		for _, l := range strings.Split(strings.TrimSpace(msg), "\n") {
			lines = append(lines, html.EscapeString(strings.TrimSpace(l)))
		}
		want := "<p>" + strings.Join(lines, "\n") + "</p>\n"
		if strings.TrimSpace(msg) == "" {
			want = ""
		}
		want = strings.NewReplacer("&#39;", "'", "&#34;", "&quot;").Replace(want)
		if got != want {
			t.Fatalf("seed %d: %q is plain, but Markdown makes %q, not %q", seed, msg, got, want)
		}
	}
	if plain < 1000 {
		t.Fatalf("only %d plain messages generated", plain)
	}
}
