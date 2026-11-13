package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/txchen/mattermostgo/internal/store"
)

// integration is an integration's server, as the Mattermost server
// calls it for buttons, menus and dialogs.
type integration struct {
	*httptest.Server
	mu        sync.Mutex
	actions   []map[string]any
	submitted []map[string]any
	base      string
	token     string
}

func newIntegration(t *testing.T, base, adminToken string) *integration {
	in := &integration{base: base, token: adminToken}
	mux := http.NewServeMux()
	mux.HandleFunc("/action", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		in.mu.Lock()
		in.actions = append(in.actions, req)
		in.mu.Unlock()
		ctx, _ := req["context"].(map[string]any)
		switch ctx["do"] {
		case "approve":
			json.NewEncoder(w).Encode(map[string]any{"update": map[string]any{"message": "Approved by the button"}, "ephemeral_text": "You approved it"})
		case "form":
			dialog := map[string]any{"trigger_id": req["trigger_id"], "url": in.URL + "/dialog", "dialog": map[string]any{
				"callback_id": "cb1", "title": "Deploy form", "introduction_text": "Where to?", "submit_label": "Deploy", "state": "s1",
				"elements": []map[string]any{
					{"display_name": "Release name", "name": "release", "type": "text", "placeholder": "v1.2"},
					{"display_name": "Environment", "name": "env", "type": "select", "options": []map[string]string{{"text": "Staging", "value": "staging"}, {"text": "Production", "value": "prod"}}, "default": "staging"},
					{"display_name": "Notify", "name": "notify", "type": "bool", "placeholder": "Tell the team", "optional": true},
				}}}
			b, _ := json.Marshal(dialog)
			r2, _ := http.NewRequest("POST", in.base+"/api/v4/actions/dialogs/open", bytes.NewReader(b))
			r2.Header.Set("Authorization", "Bearer "+in.token)
			r2.Header.Set("Content-Type", "application/json")
			if res, err := http.DefaultClient.Do(r2); err == nil {
				res.Body.Close()
			}
			w.Write([]byte(`{}`))
		default:
			w.Write([]byte(`{}`))
		}
	})
	mux.HandleFunc("/dialog", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		in.mu.Lock()
		in.submitted = append(in.submitted, req)
		in.mu.Unlock()
		if sub, _ := req["submission"].(map[string]any); sub["release"] == "bad" {
			json.NewEncoder(w).Encode(map[string]any{"errors": map[string]string{"release": "No such release"}})
			return
		}
		w.Write([]byte(`{}`))
	})
	in.Server = httptest.NewServer(mux)
	t.Cleanup(in.Close)
	return in
}

func (in *integration) last() (actions, submitted []map[string]any) {
	in.mu.Lock()
	defer in.mu.Unlock()
	return append([]map[string]any(nil), in.actions...), append([]map[string]any(nil), in.submitted...)
}

// Buttons and menus of an attachment reach the integration, its answer
// updates the post and shows an ephemeral message, and a dialog it opens
// shows, checks its fields and submits.
func TestInteractiveMessages(t *testing.T) {
	base := serverURL(t)
	admin, _ := login(t, base, "admin")
	in := newIntegration(t, base, admin.Token())
	c := newClient(t)
	s := c.loginAs(base, "alice")
	town := channelByName(s, "town-square")
	c.a.OpenChannel(s, town)
	c.until("town loaded", func() bool { tl := s.Store.Timelines[town]; return tl != nil && !tl.Loading })

	tag := fmt.Sprint(time.Now().UnixNano() % 1000000)
	action := func(id, name string, do string) map[string]any {
		return map[string]any{"id": id, "name": name, "integration": map[string]any{"url": in.URL + "/action", "context": map[string]string{"do": do}}}
	}
	sel := action("pick"+tag, "Pick a size "+tag, "pick")
	sel["type"], sel["options"] = "select", []map[string]string{{"text": "Small", "value": "s"}, {"text": "Large", "value": "l"}}
	body := map[string]any{"channel_id": town, "message": "", "props": map[string]any{"attachments": []map[string]any{{
		"title": "Deploy " + tag, "text": "Ready to ship",
		"actions": []map[string]any{action("approve"+tag, "Approve "+tag, "approve"), action("form"+tag, "Open form "+tag, "form"), sel},
	}}}}
	var post map[string]any
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+"/api/v4/posts", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+admin.Token())
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode >= 300 {
		t.Fatalf("posting: %v %v", res, err)
	}
	json.NewDecoder(res.Body).Decode(&post)
	res.Body.Close()
	id := store.PostID(post["id"].(string))
	c.until("buttons", func() bool { return c.has("Deploy "+tag)() && c.has("Approve "+tag)() })

	c.click("Pick a size " + tag)
	if err := c.tt.ChooseMenuItem("Large"); err != nil {
		t.Fatalf("%v in %q", err, c.tt.Menu())
	}
	c.until("menu reached the integration", func() bool {
		acts, _ := in.last()
		return len(acts) > 0 && acts[len(acts)-1]["context"].(map[string]any)["selected_option"] == "l"
	})

	c.click("Approve " + tag)
	c.until("post updated", func() bool { p := s.Store.Posts[id]; return p != nil && p.Message == "Approved by the button" })
	c.until("ephemeral", c.has("You approved it"))
	c.until("only visible to you", c.has("(Only visible to you)"))

	// The update dropped the buttons; a new post brings the form.
	form := map[string]any{"channel_id": town, "props": map[string]any{"attachments": []map[string]any{{"text": "Form " + tag, "actions": []map[string]any{action("form2"+tag, "Form button "+tag, "form")}}}}}
	b, _ = json.Marshal(form)
	req, _ = http.NewRequest("POST", base+"/api/v4/posts", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+admin.Token())
	req.Header.Set("Content-Type", "application/json")
	if res, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	c.until("form button", c.has("Form "+tag))
	c.click("Form button " + tag)
	c.until("dialog", func() bool { return c.has("Deploy form")() && c.has("Where to?")() })
	c.click("Deploy")
	c.until("required field", c.has("This field is required."))
	c.click("Release name")
	c.tt.Type("bad")
	c.click("Deploy")
	c.until("integration error", c.has("No such release"))
	c.click("Release name")
	c.tt.Key(cmdA())
	c.tt.Type("v9")
	c.click("Deploy")
	c.until("submitted", func() bool {
		_, subs := in.last()
		if len(subs) < 2 {
			return false
		}
		sub := subs[len(subs)-1]["submission"].(map[string]any)
		return sub["release"] == "v9" && sub["env"] == "staging" && subs[len(subs)-1]["state"] == "s1"
	})
	c.until("dialog closed", func() bool { return !c.has("Deploy form")() })
}

func cmdA() (ui.Modifiers, ui.Key) { return ui.Cmd, ui.KeyA }
