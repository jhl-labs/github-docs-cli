package github

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// User is a GitHub account reference.
type User struct {
	Login string `json:"login"`
}

// Label is an issue label.
type Label struct {
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// Issue is a GitHub issue (pull requests are filtered out by the commands).
type Issue struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      string  `json:"body,omitempty"`
	State     string  `json:"state"`
	HTMLURL   string  `json:"html_url,omitempty"`
	Labels    []Label `json:"labels,omitempty"`
	User      *User   `json:"user,omitempty"`
	Assignees []User  `json:"assignees,omitempty"`
	Comments  int     `json:"comments"`
	CreatedAt string  `json:"created_at,omitempty"`
	UpdatedAt string  `json:"updated_at,omitempty"`
	// PullRequest is set by the API when the "issue" is actually a PR.
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request,omitempty"`
}

// IsPullRequest reports whether this issue object is really a pull request.
func (i Issue) IsPullRequest() bool { return i.PullRequest != nil }

// Comment is a comment on an issue.
type Comment struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	User      *User  `json:"user,omitempty"`
	HTMLURL   string `json:"html_url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ListIssuesOptions filters an issue listing.
type ListIssuesOptions struct {
	State    string // open|closed|all (default open)
	Labels   string // comma-separated label names
	Assignee string // login, or "*"/"none"
	Sort     string // created|updated|comments
	Limit    int
}

// ListIssues returns issues for a repository.
func (c *Client) ListIssues(ctx context.Context, owner, repo string, opts ListIssuesOptions) ([]Issue, error) {
	q := url.Values{}
	if opts.State != "" {
		q.Set("state", opts.State)
	}
	if opts.Labels != "" {
		q.Set("labels", opts.Labels)
	}
	if opts.Assignee != "" {
		q.Set("assignee", opts.Assignee)
	}
	if opts.Sort != "" {
		q.Set("sort", opts.Sort)
	}
	if opts.Limit > 0 {
		q.Set("per_page", strconv.Itoa(min(opts.Limit, 100)))
	}
	var out []Issue
	if err := c.doJSON(ctx, "GET", c.repoPath(owner, repo, "issues"), q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetIssue fetches a single issue by number.
func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	var out Issue
	path := c.repoPath(owner, repo, "issues/"+strconv.Itoa(number))
	if err := c.doJSON(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListIssueComments returns the comments on an issue.
func (c *Client) ListIssueComments(ctx context.Context, owner, repo string, number int) ([]Comment, error) {
	q := url.Values{}
	q.Set("per_page", "100")
	var out []Comment
	path := c.repoPath(owner, repo, "issues/"+strconv.Itoa(number)+"/comments")
	if err := c.doJSON(ctx, "GET", path, q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateIssueInput describes a new issue.
type CreateIssueInput struct {
	Title     string
	Body      string
	Labels    []string
	Assignees []string
}

// CreateIssue opens a new issue.
func (c *Client) CreateIssue(ctx context.Context, owner, repo string, in CreateIssueInput) (*Issue, error) {
	payload := map[string]any{"title": in.Title}
	if in.Body != "" {
		payload["body"] = in.Body
	}
	if len(in.Labels) > 0 {
		payload["labels"] = in.Labels
	}
	if len(in.Assignees) > 0 {
		payload["assignees"] = in.Assignees
	}
	var out Issue
	if err := c.doJSON(ctx, "POST", c.repoPath(owner, repo, "issues"), nil, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateIssueInput describes an edit to an issue. Empty fields are left
// unchanged; State must be "open", "closed", or empty.
type UpdateIssueInput struct {
	Title string
	Body  *string // pointer so an empty body can be set deliberately
	State string
}

// UpdateIssue edits an existing issue's title, body, and/or state.
func (c *Client) UpdateIssue(ctx context.Context, owner, repo string, number int, in UpdateIssueInput) (*Issue, error) {
	payload := map[string]any{}
	if in.Title != "" {
		payload["title"] = in.Title
	}
	if in.Body != nil {
		payload["body"] = *in.Body
	}
	if in.State != "" {
		payload["state"] = in.State
	}
	var out Issue
	path := c.repoPath(owner, repo, "issues/"+strconv.Itoa(number))
	if err := c.doJSON(ctx, "PATCH", path, nil, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddIssueComment posts a comment on an issue.
func (c *Client) AddIssueComment(ctx context.Context, owner, repo string, number int, body string) (*Comment, error) {
	payload := map[string]any{"body": body}
	var out Comment
	path := c.repoPath(owner, repo, "issues/"+strconv.Itoa(number)+"/comments")
	if err := c.doJSON(ctx, "POST", path, nil, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddIssueLabels adds labels to an issue and returns the resulting label set.
func (c *Client) AddIssueLabels(ctx context.Context, owner, repo string, number int, names []string) ([]Label, error) {
	payload := map[string]any{"labels": names}
	var out []Label
	path := c.repoPath(owner, repo, "issues/"+strconv.Itoa(number)+"/labels")
	if err := c.doJSON(ctx, "POST", path, nil, payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveIssueLabel removes a single label from an issue.
func (c *Client) RemoveIssueLabel(ctx context.Context, owner, repo string, number int, name string) error {
	path := c.repoPath(owner, repo, "issues/"+strconv.Itoa(number)+"/labels/"+url.PathEscape(name))
	return c.doJSON(ctx, "DELETE", path, nil, nil, nil)
}

// repoPath builds "/repos/{owner}/{repo}/{suffix}" with each segment escaped.
func (c *Client) repoPath(owner, repo, suffix string) string {
	p := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	if suffix != "" {
		p += "/" + suffix
	}
	return p
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// joinLabels renders label names for text output.
func joinLabels(labels []Label) string {
	if len(labels) == 0 {
		return ""
	}
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}

// LabelNames returns just the names of a label slice.
func LabelNames(labels []Label) string { return joinLabels(labels) }
