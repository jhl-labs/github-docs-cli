package github

import "context"

// Discussion is a GitHub Discussion.
type Discussion struct {
	ID       string              `json:"id"` // GraphQL node ID
	Number   int                 `json:"number"`
	Title    string              `json:"title"`
	Body     string              `json:"body,omitempty"`
	URL      string              `json:"url,omitempty"`
	Category DiscussionCategory  `json:"category"`
	Author   *Author             `json:"author,omitempty"`
	Comments []DiscussionComment `json:"-"`
}

// Author is the GraphQL actor login wrapper.
type Author struct {
	Login string `json:"login"`
}

// DiscussionCategory is a discussion category.
type DiscussionCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// DiscussionComment is a top-level comment on a discussion.
type DiscussionComment struct {
	ID     string  `json:"id"`
	Body   string  `json:"body"`
	URL    string  `json:"url,omitempty"`
	Author *Author `json:"author,omitempty"`
}

// ListDiscussions returns the most recent discussions in a repository.
func (c *Client) ListDiscussions(ctx context.Context, owner, repo string, limit int) ([]Discussion, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	const query = `
query($owner:String!, $repo:String!, $limit:Int!) {
  repository(owner:$owner, name:$repo) {
    discussions(first:$limit, orderBy:{field:UPDATED_AT, direction:DESC}) {
      nodes {
        id number title url
        category { id name }
        author { login }
      }
    }
  }
}`
	vars := map[string]any{"owner": owner, "repo": repo, "limit": limit}
	var out struct {
		Repository struct {
			Discussions struct {
				Nodes []Discussion `json:"nodes"`
			} `json:"discussions"`
		} `json:"repository"`
	}
	if err := c.doGraphQL(ctx, query, vars, &out); err != nil {
		return nil, err
	}
	return out.Repository.Discussions.Nodes, nil
}

// GetDiscussion fetches a discussion (with its top-level comments) by number.
func (c *Client) GetDiscussion(ctx context.Context, owner, repo string, number int) (*Discussion, error) {
	const query = `
query($owner:String!, $repo:String!, $number:Int!) {
  repository(owner:$owner, name:$repo) {
    discussion(number:$number) {
      id number title body url
      category { id name }
      author { login }
      comments(first:100) { nodes { id body url author { login } } }
    }
  }
}`
	vars := map[string]any{"owner": owner, "repo": repo, "number": number}
	var out struct {
		Repository struct {
			Discussion struct {
				Discussion
				Comments struct {
					Nodes []DiscussionComment `json:"nodes"`
				} `json:"comments"`
			} `json:"discussion"`
		} `json:"repository"`
	}
	if err := c.doGraphQL(ctx, query, vars, &out); err != nil {
		return nil, err
	}
	d := out.Repository.Discussion.Discussion
	d.Comments = out.Repository.Discussion.Comments.Nodes
	return &d, nil
}

// ListDiscussionCategories returns the repository's discussion categories.
func (c *Client) ListDiscussionCategories(ctx context.Context, owner, repo string) ([]DiscussionCategory, error) {
	const query = `
query($owner:String!, $repo:String!) {
  repository(owner:$owner, name:$repo) {
    discussionCategories(first:50) { nodes { id name description } }
  }
}`
	vars := map[string]any{"owner": owner, "repo": repo}
	var out struct {
		Repository struct {
			DiscussionCategories struct {
				Nodes []DiscussionCategory `json:"nodes"`
			} `json:"discussionCategories"`
		} `json:"repository"`
	}
	if err := c.doGraphQL(ctx, query, vars, &out); err != nil {
		return nil, err
	}
	return out.Repository.DiscussionCategories.Nodes, nil
}

// repoIDs holds the GraphQL node IDs needed to create a discussion.
func (c *Client) repoAndCategoryID(ctx context.Context, owner, repo, categoryName string) (repoID, categoryID string, err error) {
	const query = `
query($owner:String!, $repo:String!) {
  repository(owner:$owner, name:$repo) {
    id
    discussionCategories(first:50) { nodes { id name } }
  }
}`
	vars := map[string]any{"owner": owner, "repo": repo}
	var out struct {
		Repository struct {
			ID                   string `json:"id"`
			DiscussionCategories struct {
				Nodes []DiscussionCategory `json:"nodes"`
			} `json:"discussionCategories"`
		} `json:"repository"`
	}
	if err := c.doGraphQL(ctx, query, vars, &out); err != nil {
		return "", "", err
	}
	repoID = out.Repository.ID
	for _, cat := range out.Repository.DiscussionCategories.Nodes {
		if cat.Name == categoryName {
			categoryID = cat.ID
			break
		}
	}
	return repoID, categoryID, nil
}

// CreateDiscussion opens a new discussion in the named category.
func (c *Client) CreateDiscussion(ctx context.Context, owner, repo, categoryName, title, body string) (*Discussion, error) {
	repoID, categoryID, err := c.repoAndCategoryID(ctx, owner, repo, categoryName)
	if err != nil {
		return nil, err
	}
	if categoryID == "" {
		return nil, &APIError{StatusCode: 404, Message: "discussion category not found: " + categoryName}
	}
	const mutation = `
mutation($repoId:ID!, $categoryId:ID!, $title:String!, $body:String!) {
  createDiscussion(input:{repositoryId:$repoId, categoryId:$categoryId, title:$title, body:$body}) {
    discussion { id number title url category { id name } author { login } }
  }
}`
	vars := map[string]any{"repoId": repoID, "categoryId": categoryID, "title": title, "body": body}
	var out struct {
		CreateDiscussion struct {
			Discussion Discussion `json:"discussion"`
		} `json:"createDiscussion"`
	}
	if err := c.doGraphQL(ctx, mutation, vars, &out); err != nil {
		return nil, err
	}
	return &out.CreateDiscussion.Discussion, nil
}

// AddDiscussionComment posts a top-level comment on a discussion (by number).
func (c *Client) AddDiscussionComment(ctx context.Context, owner, repo string, number int, body string) (*DiscussionComment, error) {
	d, err := c.GetDiscussion(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	const mutation = `
mutation($discussionId:ID!, $body:String!) {
  addDiscussionComment(input:{discussionId:$discussionId, body:$body}) {
    comment { id body url author { login } }
  }
}`
	vars := map[string]any{"discussionId": d.ID, "body": body}
	var out struct {
		AddDiscussionComment struct {
			Comment DiscussionComment `json:"comment"`
		} `json:"addDiscussionComment"`
	}
	if err := c.doGraphQL(ctx, mutation, vars, &out); err != nil {
		return nil, err
	}
	return &out.AddDiscussionComment.Comment, nil
}
