package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListIssuesQueryAndFilter(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode([]Issue{
			{Number: 1, Title: "Doc", State: "open"},
			{Number: 2, Title: "A PR", State: "open", PullRequest: &struct {
				URL string `json:"url"`
			}{URL: "x"}},
		})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	issues, err := cl.ListIssues(context.Background(), "o", "r", ListIssuesOptions{
		State: "open", Labels: "docs", Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues", len(issues))
	}
	if !issues[1].IsPullRequest() {
		t.Error("issue 2 should be detected as a pull request")
	}
	for _, want := range []string{"state=open", "labels=docs", "per_page=10"} {
		if !contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestCreateIssuePayload(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		json.NewEncoder(w).Encode(Issue{Number: 7, Title: "T", HTMLURL: "http://x/7"})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	iss, err := cl.CreateIssue(context.Background(), "o", "r", CreateIssueInput{
		Title: "T", Body: "B", Labels: []string{"docs"},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if body["title"] != "T" || body["body"] != "B" {
		t.Errorf("payload = %v", body)
	}
	labels, _ := body["labels"].([]any)
	if len(labels) != 1 || labels[0] != "docs" {
		t.Errorf("labels = %v", body["labels"])
	}
	if iss.Number != 7 {
		t.Errorf("number = %d", iss.Number)
	}
}

func TestUpdateIssueOnlySetsProvidedFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		json.NewEncoder(w).Encode(Issue{Number: 3, State: "closed"})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	_, err := cl.UpdateIssue(context.Background(), "o", "r", 3, UpdateIssueInput{State: "closed"})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if _, ok := body["title"]; ok {
		t.Errorf("title should not be present when empty: %v", body)
	}
	if _, ok := body["body"]; ok {
		t.Errorf("body should not be present when nil: %v", body)
	}
	if body["state"] != "closed" {
		t.Errorf("state = %v", body["state"])
	}
}

func TestRemoveIssueLabelEscapesName(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.EscapedPath(), r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	if err := cl.RemoveIssueLabel(context.Background(), "o", "r", 4, "needs review"); err != nil {
		t.Fatalf("RemoveIssueLabel: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/repos/o/r/issues/4/labels/needs%20review" {
		t.Errorf("path = %q", gotPath)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return len(sub) == 0
}
