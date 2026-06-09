package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFileDecodesBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := base64.StdEncoding.EncodeToString([]byte("# Title\n"))
		json.NewEncoder(w).Encode(ContentEntry{
			Type: "file", Name: "guide.md", Path: "docs/guide.md",
			SHA: "abc", Encoding: "base64", Content: enc[:5] + "\n" + enc[5:],
		})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	f, err := cl.GetFile(context.Background(), "o", "r", "docs/guide.md", "")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	got, err := f.Decoded()
	if err != nil {
		t.Fatalf("Decoded: %v", err)
	}
	if got != "# Title\n" {
		t.Errorf("decoded = %q", got)
	}
}

func TestPutFileCreateVsUpdate(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		json.NewEncoder(w).Encode(FileCommit{})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)

	// Create: no SHA in payload.
	body = nil
	if _, err := cl.PutFile(context.Background(), "o", "r", PutFileInput{
		Path: "docs/a.md", Content: "hi", Message: "m",
	}); err != nil {
		t.Fatalf("PutFile create: %v", err)
	}
	if _, ok := body["sha"]; ok {
		t.Errorf("create payload should omit sha: %v", body)
	}
	if dec, _ := base64.StdEncoding.DecodeString(body["content"].(string)); string(dec) != "hi" {
		t.Errorf("content not base64-encoded correctly: %v", body["content"])
	}

	// Update: SHA present.
	body = nil
	if _, err := cl.PutFile(context.Background(), "o", "r", PutFileInput{
		Path: "docs/a.md", Content: "hi2", Message: "m", SHA: "deadbeef",
	}); err != nil {
		t.Fatalf("PutFile update: %v", err)
	}
	if body["sha"] != "deadbeef" {
		t.Errorf("update payload sha = %v", body["sha"])
	}
}

func TestEnablePagesUpdatesWhenExisting(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(PagesInfo{Status: "built", HTMLURL: "http://pages"})
		case http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	info, err := cl.EnablePages(context.Background(), "o", "r", "main", "/docs")
	if err != nil {
		t.Fatalf("EnablePages: %v", err)
	}
	if info.HTMLURL != "http://pages" {
		t.Errorf("html url = %q", info.HTMLURL)
	}
	// Existing pages -> GET, then PUT (update), then GET (re-fetch).
	if len(methods) < 2 || methods[1] != http.MethodPut {
		t.Errorf("methods = %v, want a PUT after the initial GET", methods)
	}
}

func TestGetTreeRecursiveQuery(t *testing.T) {
	var gotQuery, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery, gotPath = r.URL.RawQuery, r.URL.Path
		json.NewEncoder(w).Encode(Tree{
			SHA:  "root",
			Tree: []TreeEntry{{Path: "docs/a.md", Type: "blob"}, {Path: "docs", Type: "tree"}},
		})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	tree, err := cl.GetTree(context.Background(), "o", "r", "main", true)
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}
	if gotPath != "/repos/o/r/git/trees/main" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "recursive=1" {
		t.Errorf("query = %q", gotQuery)
	}
	if len(tree.Tree) != 2 {
		t.Errorf("entries = %d", len(tree.Tree))
	}
}
