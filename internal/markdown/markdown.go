// Package markdown turns Mattermost's Markdown into the CommonMark and GFM
// that mygo's ui/markdown shows: emoji names become emoji, and @mentions
// and ~channel links become links the client handles.
package markdown

//go:generate go run gen_emoji.go

import (
	"net/url"
	"sort"
	"strings"
)

const (
	// MentionScheme and ChannelScheme start the links Prepare makes.
	MentionScheme = "mm-mention:"
	ChannelScheme = "mm-channel:"
	// EmojiScheme starts the links of custom emoji.
	EmojiScheme = "mm-emoji:"
)

// Emoji returns the text of an emoji by its Mattermost name.
func Emoji(name string) (string, bool) {
	e, ok := emojis[name]
	return e, ok
}

// EmojiCategory is a group of the emoji picker, its emoji in order.
type EmojiCategory struct {
	Name   string
	Emojis []string
}

// EmojiCategories returns the groups of the picker, as the webapp orders them.
func EmojiCategories() []EmojiCategory { return emojiCategories }

// SkinTones are the suffixes of the skin tone variants, lightest first.
var SkinTones = []string{"_light_skin_tone", "_medium_light_skin_tone", "_medium_skin_tone", "_medium_dark_skin_tone", "_dark_skin_tone"}

// WithSkinTone returns the variant of an emoji in a tone (1 to 5), or the
// emoji itself when it has none or tone is 0.
func WithSkinTone(name string, tone int) string {
	if tone < 1 || tone > len(SkinTones) || !skinToned[name] {
		return name
	}
	return name + SkinTones[tone-1]
}

// EmojiNames returns the names starting with prefix, sorted, at most n.
func EmojiNames(prefix string, n int) []string {
	var out []string
	for name := range emojis {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) < len(out[j])
		}
		return out[i] < out[j]
	})
	return out[:min(n, len(out))]
}

// Prepare rewrites a post's message for the Markdown view. Code spans and
// code blocks stay as they are.
func Prepare(msg string) string { return PrepareWith(msg, nil) }

// PrepareWith is Prepare, with custom naming the custom emoji of the
// server, which become links to EmojiScheme for the view to draw.
func PrepareWith(msg string, custom func(name string) bool) string {
	if !strings.ContainsAny(msg, ":@~") {
		return msg
	}
	var b strings.Builder
	b.Grow(len(msg) + 32)
	fence := ""
	for line := range strings.Lines(msg) {
		trimmed := strings.TrimLeft(line, " ")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			b.WriteString(line)
			continue
		}
		if f := fenceOf(trimmed); f != "" && len(line)-len(trimmed) < 4 {
			fence = f
			b.WriteString(line)
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			b.WriteString(line)
			continue
		}
		prepareLine(&b, line, custom)
	}
	return b.String()
}

func fenceOf(line string) string {
	for _, f := range []string{"```", "~~~"} {
		if strings.HasPrefix(line, f) {
			n := len(f)
			for n < len(line) && line[n] == f[0] {
				n++
			}
			return line[:n]
		}
	}
	return ""
}

func prepareLine(b *strings.Builder, s string, custom func(string) bool) {
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '`':
			n := run(s, i, '`')
			if end := strings.Index(s[i+n:], s[i:i+n]); end >= 0 {
				b.WriteString(s[i : i+n+end+n])
				i += n + end + n
				continue
			}
			b.WriteString(s[i : i+n])
			i += n
			continue
		case c == '\\' && i+1 < len(s):
			b.WriteString(s[i : i+2])
			i += 2
			continue
		case c == '[' || c == '<' || (c == 'h' && strings.HasPrefix(s[i:], "http")):
			// Links and URLs keep their text: a username in a URL is not a mention.
			if end := linkEnd(s, i); end > i {
				b.WriteString(s[i:end])
				i = end
				continue
			}
		case c == ':':
			if end := strings.IndexByte(s[i+1:], ':'); end > 0 {
				name := s[i+1 : i+1+end]
				if e, ok := emojis[name]; ok {
					b.WriteString(e)
					i += end + 2
					continue
				}
				if custom != nil && emojiName(name) && custom(name) {
					// A zero-width space is the text of the link, which
					// the view draws the emoji before.
					b.WriteString("[\u200b](" + EmojiScheme + name + ")")
					i += end + 2
					continue
				}
			}
		case (c == '@' || c == '~') && (i == 0 || !isNameByte(s[i-1]) && s[i-1] != c):
			n := nameLen(s[i+1:])
			if n > 0 {
				name := s[i+1 : i+1+n]
				scheme := MentionScheme
				if c == '~' {
					scheme = ChannelScheme
				}
				b.WriteString("[")
				b.WriteByte(c)
				b.WriteString(strings.ReplaceAll(name, "_", "\\_"))
				b.WriteString("](")
				b.WriteString(scheme)
				b.WriteString(url.PathEscape(name))
				b.WriteString(")")
				i += 1 + n
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
}

func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// linkEnd returns the end of a Markdown link, an autolink or a bare URL
// starting at i, or i when there is none.
func linkEnd(s string, i int) int {
	switch s[i] {
	case '[':
		close := strings.Index(s[i:], "](")
		if close < 0 {
			return i
		}
		end := strings.IndexByte(s[i+close:], ')')
		if end < 0 {
			return i
		}
		return i + close + end + 1
	case '<':
		end := strings.IndexByte(s[i:], '>')
		if end < 0 || !strings.Contains(s[i:i+end], ":") {
			return i
		}
		return i + end + 1
	default:
		if !strings.HasPrefix(s[i:], "http://") && !strings.HasPrefix(s[i:], "https://") {
			return i
		}
		end := strings.IndexAny(s[i:], " \t\n)")
		if end < 0 {
			return len(s)
		}
		return i + end
	}
}

func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.'
}

// nameLen is the length of the username or channel name at the start of
// s; a trailing period ends a sentence, not the name.
func nameLen(s string) int {
	n := 0
	for n < len(s) && isNameByte(s[n]) {
		n++
	}
	for n > 0 && s[n-1] == '.' {
		n--
	}
	return n
}

// emojiName reports whether s can name an emoji: letters, digits, _, -
// and +.
func emojiName(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '+') {
			return false
		}
	}
	return s != ""
}
