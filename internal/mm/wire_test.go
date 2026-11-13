package mm

import (
	"encoding/json"
	"testing"

	"github.com/txchen/mattermostgo/internal/store"
)

// The metadata of a post with a link, as the dev server sent it.
const linkPost = `{"id":"p","message":"see https://github.com/mattermost/mattermost","metadata":{"embeds":[{"type":"opengraph","url":"https://github.com/mattermost/mattermost","data":{"type":"object","url":"https://github.com/mattermost/mattermost","title":"GitHub - mattermost/mattermost","description":"Mattermost is an open source platform","site_name":"GitHub","images":[{"url":"https://repository-images.githubusercontent.com/37448451/e844","secure_url":"","width":0,"height":0}]}}],"images":{"https://repository-images.githubusercontent.com/37448451/e844":{"width":1280,"height":640,"format":"png"}}}}`

const webhookPost = `{"id":"w","message":"","props":{"from_webhook":"true","override_icon_url":"https://example.com/i.png","attachments":[{"fallback":"Build failed","color":"danger","pretext":"CI","title":"Build 42","title_link":"https://ci.example.com/42","text":"**main** broke","fields":[{"title":"Branch","value":"main","short":true},{"title":"Took","value":93,"short":true},{"title":"Log","value":"long","short":false}],"footer":"ci"}]},"metadata":{"embeds":[{"type":"message_attachment"}]}}`

func TestPostPreviews(t *testing.T) {
	var w wirePost
	if err := json.Unmarshal([]byte(linkPost), &w); err != nil {
		t.Fatal(err)
	}
	p := w.post()
	want := store.Embed{URL: "https://github.com/mattermost/mattermost", SiteName: "GitHub", Title: "GitHub - mattermost/mattermost",
		Description: "Mattermost is an open source platform", Image: &store.Image{URL: "https://repository-images.githubusercontent.com/37448451/e844", Width: 1280, Height: 640}}
	if p.Embed == nil || p.Embed.Title != want.Title || p.Embed.SiteName != want.SiteName || *p.Embed.Image != *want.Image {
		t.Errorf("embed %+v", p.Embed)
	}

	w = wirePost{}
	if err := json.Unmarshal([]byte(webhookPost), &w); err != nil {
		t.Fatal(err)
	}
	p = w.post()
	if p.Embed != nil {
		t.Errorf("an attachment post shows an embed too: %+v", p.Embed)
	}
	if p.Props.OverrideIconURL != "https://example.com/i.png" || !p.Props.FromBot {
		t.Errorf("props %+v", p.Props)
	}
	if len(p.Attachments) != 1 {
		t.Fatalf("attachments %+v", p.Attachments)
	}
	a := p.Attachments[0]
	if a.Title != "Build 42" || a.Color != "danger" || a.Text != "**main** broke" || len(a.Fields) != 3 {
		t.Errorf("attachment %+v", a)
	}
	if f := a.Fields[1]; f.Value != "93" || !f.Short {
		t.Errorf("number field %+v", f)
	}
	if a.Fields[2].Short {
		t.Error("long field read as short")
	}
}
