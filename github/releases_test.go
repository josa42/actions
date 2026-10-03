package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sync"
	"testing"
)

func TestTags(t *testing.T) {
	f, c := newFakeAPI(t)
	f.json("GET /repos/o/r/tags", []map[string]string{{"name": "v1.0.0"}, {"name": "v1.1.0"}})

	got, err := c.Tags(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"v1.0.0", "v1.1.0"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTagExists(t *testing.T) {
	f, c := newFakeAPI(t)
	f.json("GET /repos/o/r/git/ref/tags/v1.0.0", map[string]string{"ref": "refs/tags/v1.0.0"})

	for tag, want := range map[string]bool{"v1.0.0": true, "v2.0.0": false} {
		got, err := c.TagExists(context.Background(), repo, tag)
		if err != nil || got != want {
			t.Errorf("TagExists(%s) = %v, %v", tag, got, err)
		}
	}
}

func TestDefaultBranch(t *testing.T) {
	f, c := newFakeAPI(t)
	f.json("GET /repos/o/r", map[string]string{"default_branch": "main"})

	if got, err := c.DefaultBranch(context.Background(), repo); err != nil || got != "main" {
		t.Errorf("DefaultBranch = %q, %v", got, err)
	}
}

// fakeReleases serves one optional release and records writes.
type fakeReleases struct {
	f       *fakeAPI
	mu      sync.Mutex
	release map[string]any
	created map[string]any
	updated map[string]any
	deleted []string
	uploads map[string]string
}

func newFakeReleases(t *testing.T, existing map[string]any, draft bool) (*fakeReleases, *Client) {
	f, c := newFakeAPI(t)
	fr := &fakeReleases{f: f, release: existing, uploads: map[string]string{}}
	uploadURL := f.srv.URL + "/uploads/repos/o/r/releases/1/assets{?name,label}"

	if existing != nil {
		existing["upload_url"] = uploadURL
		if draft {
			f.json("GET /repos/o/r/releases", []any{existing})
		} else {
			f.json("GET /repos/o/r/releases/tags/v1.0.0", existing)
		}
	} else {
		f.json("GET /repos/o/r/releases", []any{})
	}

	write := func(dst *map[string]any) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fr.mu.Lock()
			defer fr.mu.Unlock()
			json.NewDecoder(r.Body).Decode(dst)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "html_url": "https://github.com/o/r/releases/tag/v1.0.0", "upload_url": uploadURL})
		}
	}
	f.handle("POST /repos/o/r/releases", write(&fr.created))
	f.handle("PATCH /repos/o/r/releases/1", write(&fr.updated))
	f.handle("DELETE /repos/o/r/releases/assets/10", func(w http.ResponseWriter, r *http.Request) {
		fr.mu.Lock()
		defer fr.mu.Unlock()
		fr.deleted = append(fr.deleted, "a.zip")
		w.WriteHeader(http.StatusNoContent)
	})
	f.handle("POST /uploads/repos/o/r/releases/1/assets", func(w http.ResponseWriter, r *http.Request) {
		fr.mu.Lock()
		defer fr.mu.Unlock()
		if ct := r.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("Content-Type = %q", ct)
		}
		data, _ := io.ReadAll(r.Body)
		fr.uploads[r.URL.Query().Get("name")] = string(data)
		io.WriteString(w, `{"id":20}`)
	})
	return fr, c
}

func TestPublishReleaseCreates(t *testing.T) {
	fr, c := newFakeReleases(t, nil, false)

	r, err := c.PublishRelease(context.Background(), repo,
		ReleaseOptions{TagName: "v1.0.0", Body: "- notes"},
		[]AssetFile{{"a.zip", []byte("zip")}, {"b c.js", []byte("js")}})
	if err != nil {
		t.Fatal(err)
	}
	if r.HTMLURL == "" {
		t.Errorf("release = %+v", r)
	}
	if fr.created["tag_name"] != "v1.0.0" || fr.created["name"] != "v1.0.0" || fr.created["body"] != "- notes" || fr.created["generate_release_notes"] != nil {
		t.Errorf("created = %v", fr.created)
	}
	if fr.uploads["a.zip"] != "zip" || fr.uploads["b c.js"] != "js" {
		t.Errorf("uploads = %v", fr.uploads)
	}
}

func TestPublishReleaseGeneratesNotes(t *testing.T) {
	fr, c := newFakeReleases(t, nil, false)

	if _, err := c.PublishRelease(context.Background(), repo, ReleaseOptions{TagName: "v1.0.0"}, nil); err != nil {
		t.Fatal(err)
	}
	if fr.created["generate_release_notes"] != true || fr.created["body"] != nil {
		t.Errorf("created = %v", fr.created)
	}
}

func TestPublishReleaseUpdatesAndReplacesAssets(t *testing.T) {
	for _, draft := range []bool{false, true} {
		existing := map[string]any{
			"id":       1,
			"tag_name": "v1.0.0",
			"assets":   []any{map[string]any{"id": 10, "name": "a.zip"}, map[string]any{"id": 11, "name": "keep.txt"}},
		}
		fr, c := newFakeReleases(t, existing, draft)

		_, err := c.PublishRelease(context.Background(), repo,
			ReleaseOptions{TagName: "v1.0.0", Body: "- notes", Prerelease: true},
			[]AssetFile{{"a.zip", []byte("new")}})
		if err != nil {
			t.Fatal(err)
		}
		if fr.created != nil {
			t.Errorf("draft=%v: created a second release", draft)
		}
		if fr.updated["body"] != "- notes" || fr.updated["prerelease"] != true || fr.updated["generate_release_notes"] != nil {
			t.Errorf("draft=%v: updated = %v", draft, fr.updated)
		}
		if !slices.Equal(fr.deleted, []string{"a.zip"}) || fr.uploads["a.zip"] != "new" {
			t.Errorf("draft=%v: deleted = %v, uploads = %v", draft, fr.deleted, fr.uploads)
		}
	}
}

func TestPublishReleaseKeepsNotesOnUpdate(t *testing.T) {
	fr, c := newFakeReleases(t, map[string]any{"id": 1, "tag_name": "v1.0.0"}, false)

	if _, err := c.PublishRelease(context.Background(), repo, ReleaseOptions{TagName: "v1.0.0"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := fr.updated["body"]; ok {
		t.Errorf("updated = %v, want body unchanged", fr.updated)
	}
}
