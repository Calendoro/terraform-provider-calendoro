package crud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	calendoro "github.com/Calendoro/calendoro-go"
)

func def(t *testing.T, name string) ResourceDef {
	t.Helper()
	for _, d := range Resources {
		if d.Type == name {
			return d
		}
	}
	t.Fatalf("no resource %s", name)
	return ResourceDef{}
}

func stub(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) API {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := routes[r.Method+" "+r.URL.Path]; ok {
			h(w, r)
			return
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"nope"}}`))
	}))
	t.Cleanup(srv.Close)
	return API{C: calendoro.New(calendoro.WithAPIKey("k"), calendoro.WithBaseURL(srv.URL)).Client}
}

func TestTableCoversTheNamedResources(t *testing.T) {
	for _, name := range []string{"event_type", "webhook", "schedule", "managed_attribute", "managed_role", "managed_template"} {
		d := def(t, name)
		if d.CreatePath == "" || d.DeletePath == "" || d.UpdatePath == "" || (d.GetPath == "") == (d.ListPath == "") || len(d.Attrs) == 0 {
			t.Fatalf("%s is incomplete: %+v", name, d)
		}
	}
}

func TestWebhookCreateUnwrapsAndKeepsSecret(t *testing.T) {
	a := stub(t, map[string]func(http.ResponseWriter, *http.Request){
		"POST /v1/webhooks": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"endpoint":{"id":"whk_1","url":"https://x"},"secret":"whsec_1"}`))
		},
	})
	obj, secret, err := a.Create(context.Background(), def(t, "webhook"), map[string]any{"url": "https://x"})
	if err != nil || obj["id"] != "whk_1" || secret != "whsec_1" {
		t.Fatal(obj, secret, err)
	}
}

func TestGetScansListPages(t *testing.T) {
	a := stub(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET /v1/webhooks": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("cursor") == "p2" {
				_, _ = w.Write([]byte(`{"data":[{"id":"whk_2"}],"nextCursor":null}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"whk_1"}],"nextCursor":"p2"}`))
		},
	})
	d := def(t, "webhook")
	if obj, found, err := a.Get(context.Background(), d, "whk_2"); err != nil || !found || obj["id"] != "whk_2" {
		t.Fatal(obj, found, err)
	}
	if _, found, err := a.Get(context.Background(), d, "whk_gone"); err != nil || found {
		t.Fatal(found, err)
	}
}

func TestMissingIsNotAnError(t *testing.T) {
	a := stub(t, nil)
	d := def(t, "event_type")
	if _, found, err := a.Get(context.Background(), d, "x"); err != nil || found {
		t.Fatal(found, err)
	}
	if err := a.Delete(context.Background(), d, "x"); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileJSON(t *testing.T) {
	var remote any
	_ = json.Unmarshal([]byte(`{"a":1,"added":true,"list":[1,2]}`), &remote)
	prior := `{ "list": [1, 2], "a": 1 }`
	if got, _ := ReconcileJSON(prior, remote); got != prior {
		t.Fatal("server defaults must not show as drift:", got)
	}
	if got, _ := ReconcileJSON(`{"a":2}`, remote); got == `{"a":2}` {
		t.Fatal("a changed value must show as drift")
	}
	if got, _ := ReconcileJSON("", remote); got == "" {
		t.Fatal("import must fill the value")
	}
}
