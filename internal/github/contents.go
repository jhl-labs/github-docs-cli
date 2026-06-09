package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ContentEntry is one item returned by the Contents API (a file or directory).
type ContentEntry struct {
	Type    string `json:"type"` // file|dir|symlink|submodule
	Name    string `json:"name"`
	Path    string `json:"path"`
	SHA     string `json:"sha"`
	Size    int    `json:"size"`
	HTMLURL string `json:"html_url,omitempty"`
	// Content/Encoding are only populated when fetching a single file.
	Content  string `json:"content,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

// Decoded returns the decoded file content, handling base64 encoding.
func (e ContentEntry) Decoded() (string, error) {
	if e.Encoding == "base64" {
		// GitHub wraps base64 content at 60 columns.
		clean := strings.ReplaceAll(e.Content, "\n", "")
		data, err := base64.StdEncoding.DecodeString(clean)
		if err != nil {
			return "", fmt.Errorf("decoding file content: %w", err)
		}
		return string(data), nil
	}
	return e.Content, nil
}

// GetFile fetches a single file's metadata and content. It returns an error if
// the path is a directory; use ListDir for directories.
func (c *Client) GetFile(ctx context.Context, owner, repo, path, ref string) (*ContentEntry, error) {
	data, err := c.rawContents(ctx, owner, repo, path, ref)
	if err != nil {
		return nil, err
	}
	// A file path returns an object; a directory returns an array.
	var entry ContentEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("path %q is not a file (got a directory?): %w", path, err)
	}
	return &entry, nil
}

// ListDir lists the entries of a directory path (non-recursive).
func (c *Client) ListDir(ctx context.Context, owner, repo, path, ref string) ([]ContentEntry, error) {
	data, err := c.rawContents(ctx, owner, repo, path, ref)
	if err != nil {
		return nil, err
	}
	var entries []ContentEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		// A single-file response is an object, not an array.
		var single ContentEntry
		if json.Unmarshal(data, &single) == nil && single.Type == "file" {
			return []ContentEntry{single}, nil
		}
		return nil, fmt.Errorf("listing %q: %w", path, err)
	}
	return entries, nil
}

func (c *Client) rawContents(ctx context.Context, owner, repo, path, ref string) (json.RawMessage, error) {
	q := url.Values{}
	if ref != "" {
		q.Set("ref", ref)
	}
	apiPath := c.repoPath(owner, repo, "contents/"+escapePath(path))
	var raw json.RawMessage
	if err := c.doJSON(ctx, "GET", apiPath, q, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// PutFileInput describes a file create/update via the Contents API.
type PutFileInput struct {
	Path    string
	Content string // raw (not base64) content
	Message string // commit message
	SHA     string // blob SHA of the file being replaced (required for updates)
	Branch  string // optional target branch
}

// FileCommit is the response from a create/update/delete file operation.
type FileCommit struct {
	Content *ContentEntry `json:"content,omitempty"`
	Commit  struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
	} `json:"commit"`
}

// PutFile creates or updates a file. When SHA is empty the call creates a new
// file; pass the current blob SHA to update an existing one.
func (c *Client) PutFile(ctx context.Context, owner, repo string, in PutFileInput) (*FileCommit, error) {
	payload := map[string]any{
		"message": in.Message,
		"content": base64.StdEncoding.EncodeToString([]byte(in.Content)),
	}
	if in.SHA != "" {
		payload["sha"] = in.SHA
	}
	if in.Branch != "" {
		payload["branch"] = in.Branch
	}
	var out FileCommit
	apiPath := c.repoPath(owner, repo, "contents/"+escapePath(in.Path))
	if err := c.doJSON(ctx, "PUT", apiPath, nil, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteFile removes a file. The current blob SHA is required.
func (c *Client) DeleteFile(ctx context.Context, owner, repo, path, message, sha, branch string) (*FileCommit, error) {
	payload := map[string]any{"message": message, "sha": sha}
	if branch != "" {
		payload["branch"] = branch
	}
	var out FileCommit
	apiPath := c.repoPath(owner, repo, "contents/"+escapePath(path))
	if err := c.doJSON(ctx, "DELETE", apiPath, nil, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TreeEntry is one node of a git tree.
type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // blob|tree|commit
	SHA  string `json:"sha"`
	Size int    `json:"size,omitempty"`
}

// Tree is a git tree listing.
type Tree struct {
	SHA       string      `json:"sha"`
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// GetTree returns the git tree at a ref (branch name or SHA), optionally
// recursive. Use it to render a repository's file hierarchy.
func (c *Client) GetTree(ctx context.Context, owner, repo, ref string, recursive bool) (*Tree, error) {
	q := url.Values{}
	if recursive {
		q.Set("recursive", "1")
	}
	apiPath := c.repoPath(owner, repo, "git/trees/"+url.PathEscape(ref))
	var out Tree
	if err := c.doJSON(ctx, "GET", apiPath, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PagesInfo is the GitHub Pages configuration/status for a repository.
type PagesInfo struct {
	URL       string `json:"url,omitempty"`
	HTMLURL   string `json:"html_url,omitempty"`
	Status    string `json:"status,omitempty"`
	BuildType string `json:"build_type,omitempty"`
	Public    bool   `json:"public,omitempty"`
	Source    *struct {
		Branch string `json:"branch"`
		Path   string `json:"path"`
	} `json:"source,omitempty"`
}

// GetPages returns the Pages configuration, or a 404 APIError if Pages is not
// enabled (check with NotFound).
func (c *Client) GetPages(ctx context.Context, owner, repo string) (*PagesInfo, error) {
	var out PagesInfo
	if err := c.doJSON(ctx, "GET", c.repoPath(owner, repo, "pages"), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EnablePages enables (or, when already enabled, reconfigures) GitHub Pages to
// build from a branch/path source.
func (c *Client) EnablePages(ctx context.Context, owner, repo, branch, path string) (*PagesInfo, error) {
	source := map[string]string{"branch": branch, "path": path}
	// Try to create first; if it already exists, update the source instead.
	existing, err := c.GetPages(ctx, owner, repo)
	if err != nil && !NotFound(err) {
		return nil, err
	}
	if existing != nil {
		payload := map[string]any{"source": source}
		if err := c.doJSON(ctx, "PUT", c.repoPath(owner, repo, "pages"), nil, payload, nil); err != nil {
			return nil, err
		}
		return c.GetPages(ctx, owner, repo)
	}
	payload := map[string]any{"source": source}
	var out PagesInfo
	if err := c.doJSON(ctx, "POST", c.repoPath(owner, repo, "pages"), nil, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RequestPagesBuild triggers a new Pages build.
func (c *Client) RequestPagesBuild(ctx context.Context, owner, repo string) error {
	return c.doJSON(ctx, "POST", c.repoPath(owner, repo, "pages/builds"), nil, nil, nil)
}

// DefaultBranch returns the repository's default branch name.
func (c *Client) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.doJSON(ctx, "GET", c.repoPath(owner, repo, ""), nil, nil, &out); err != nil {
		return "", err
	}
	return out.DefaultBranch, nil
}

// escapePath percent-escapes each segment of a repo path while keeping "/".
func escapePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}
