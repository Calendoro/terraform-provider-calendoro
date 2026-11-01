// Package crud runs create, read, update and delete for the resources in the generated table. It knows nothing of Terraform, so it can be tested alone.
package crud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	calendoro "github.com/Calendoro/calendoro-go"
)

// Kind is an attribute's Terraform type.
type Kind int

const (
	KindString Kind = iota
	KindInt
	KindFloat
	KindBool
	// KindJSON is anything else: arrays and objects are JSON text, written with jsonencode().
	KindJSON
)

// AttrDef is one configurable attribute.
type AttrDef struct {
	Name     string
	Wire     string
	Kind     Kind
	Required bool
	ForceNew bool
	Doc      string
}

// ResourceDef describes one resource. Exactly one of GetPath and ListPath is set: some areas of the API have no get-by-id.
type ResourceDef struct {
	Type         string
	CreatePath   string
	UpdateMethod string
	UpdatePath   string
	DeletePath   string
	GetPath      string
	ListPath     string
	Wrap         string // the create response nests the object under this key
	Secret       string // the create response has this write-once secret beside it
	Attrs        []AttrDef
}

// API runs requests against Calendoro.
type API struct{ C *calendoro.Client }

func decode(raw []byte) (map[string]any, error) {
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return out, nil
}

// Create returns the created object and its write-once secret, if the resource has one.
func (a API) Create(ctx context.Context, d ResourceDef, body map[string]any) (map[string]any, string, error) {
	raw, err := a.C.Do(ctx, calendoro.Request{Method: "POST", Path: d.CreatePath, Body: body})
	if err != nil {
		return nil, "", err
	}
	out, err := decode(raw)
	if err != nil {
		return nil, "", err
	}
	secret := ""
	if d.Secret != "" {
		secret, _ = out[d.Secret].(string)
	}
	if d.Wrap != "" {
		inner, ok := out[d.Wrap].(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("response has no %q object", d.Wrap)
		}
		out = inner
	}
	return out, secret, nil
}

// Get finds an object by id. Found is false when it no longer exists, so Terraform can plan to recreate it.
func (a API) Get(ctx context.Context, d ResourceDef, id string) (obj map[string]any, found bool, err error) {
	if d.GetPath != "" {
		raw, err := a.C.Do(ctx, calendoro.Request{Method: "GET", Path: d.GetPath, PathParams: map[string]string{"id": id}})
		if isNotFound(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		out, err := decode(raw)
		return out, err == nil, err
	}
	query := map[string]any{}
	for {
		raw, err := a.C.Do(ctx, calendoro.Request{Method: "GET", Path: d.ListPath, Query: query})
		if err != nil {
			return nil, false, err
		}
		var page struct {
			Data       []map[string]any `json:"data"`
			NextCursor *string          `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, false, fmt.Errorf("decoding response: %w", err)
		}
		for _, item := range page.Data {
			if item["id"] == id {
				return item, true, nil
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return nil, false, nil
		}
		query["cursor"] = *page.NextCursor
	}
}

// Update changes an object and returns it.
func (a API) Update(ctx context.Context, d ResourceDef, id string, body map[string]any) (map[string]any, error) {
	raw, err := a.C.Do(ctx, calendoro.Request{Method: d.UpdateMethod, Path: d.UpdatePath, PathParams: map[string]string{"id": id}, Body: body})
	if err != nil {
		return nil, err
	}
	return decode(raw)
}

// Delete removes an object. One that is already gone is success.
func (a API) Delete(ctx context.Context, d ResourceDef, id string) error {
	_, err := a.C.Do(ctx, calendoro.Request{Method: "DELETE", Path: d.DeletePath, PathParams: map[string]string{"id": id}})
	if isNotFound(err) {
		return nil
	}
	return err
}

func isNotFound(err error) bool {
	var e *calendoro.APIError
	return errors.As(err, &e) && e.IsNotFound()
}

// Contains reports whether every value in want appears in got. Servers add defaulted keys to objects, which is not drift.
func Contains(got, want any) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range w {
			if gv, present := g[k]; !present || !Contains(gv, v) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !Contains(g[i], w[i]) {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}

// ReconcileJSON is what a JSON attribute's state should hold after a read: the prior text when the server still agrees with it, so formatting and server defaults show no diff; otherwise the server's value.
func ReconcileJSON(prior string, remote any) (string, error) {
	if prior != "" {
		var want any
		if json.Unmarshal([]byte(prior), &want) == nil && Contains(remote, want) {
			return prior, nil
		}
	}
	raw, err := json.Marshal(remote)
	return string(raw), err
}
