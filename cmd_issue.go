package main

import (
	"context"
	"flag"
	"fmt"

	"github-docs-cli/internal/github"
)

func runIssue(args []string) error {
	usage := subUsage("issue", "Manage repository issues as documents/notes.", []string{
		"list     List issues (--state, --label, --assignee, --limit)",
		"get      Show an issue (--number, --comments)",
		"create   Open an issue (--title, --body/--body-file, --label, --assignee)",
		"update   Edit an issue (--number, --title, --body, --state open|closed)",
		"comment  Add a comment (--number, --body/--body-file)",
		"label    List/add/remove labels (--number, --add, --remove)",
	})
	return dispatch("issue", args, usage, map[string]func([]string) error{
		"list":    issueList,
		"get":     issueGet,
		"create":  issueCreate,
		"update":  issueUpdate,
		"comment": issueComment,
		"label":   issueLabel,
	})
}

func issueList(args []string) error {
	fs := flag.NewFlagSet("issue list", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		state    = fs.String("state", "open", "issue state: open|closed|all")
		labels   = fs.String("label", "", "comma-separated labels to filter by")
		assignee = fs.String("assignee", "", `filter by assignee login ("none"/"*" allowed)`)
		sort     = fs.String("sort", "", "sort field: created|updated|comments")
		limit    = fs.Int("limit", 30, "maximum issues to return")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return err
	}
	cl, err := common.client()
	if err != nil {
		return err
	}

	issues, err := cl.ListIssues(context.Background(), owner, repo, github.ListIssuesOptions{
		State: *state, Labels: *labels, Assignee: *assignee, Sort: *sort, Limit: *limit,
	})
	if err != nil {
		return err
	}
	// The issues endpoint also returns PRs; drop them.
	out := issues[:0]
	for _, i := range issues {
		if !i.IsPullRequest() {
			out = append(out, i)
		}
	}

	return emit(common.output, out, func() {
		if len(out) == 0 {
			fmt.Println("no issues")
			return
		}
		for _, i := range out {
			fmt.Printf("#%-5d %-7s %s\n", i.Number, i.State, i.Title)
			if lbl := github.LabelNames(i.Labels); lbl != "" {
				fmt.Printf("       labels: %s\n", lbl)
			}
		}
		fmt.Printf("\n%d issue(s)\n", len(out))
	})
}

func issueGet(args []string) error {
	fs := flag.NewFlagSet("issue get", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		number   = fs.Int("number", 0, "issue number (required)")
		comments = fs.Bool("comments", false, "include comments")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *number == 0 {
		return fmt.Errorf("--number is required")
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return err
	}
	cl, err := common.client()
	if err != nil {
		return err
	}
	ctx := context.Background()

	issue, err := cl.GetIssue(ctx, owner, repo, *number)
	if err != nil {
		return err
	}
	var cmts []github.Comment
	if *comments {
		cmts, err = cl.ListIssueComments(ctx, owner, repo, *number)
		if err != nil {
			return err
		}
	}

	payload := struct {
		Issue    *github.Issue    `json:"issue"`
		Comments []github.Comment `json:"comments,omitempty"`
	}{issue, cmts}

	return emit(common.output, payload, func() {
		fmt.Printf("#%d [%s] %s\n", issue.Number, issue.State, issue.Title)
		if issue.User != nil {
			fmt.Printf("author:  %s\n", issue.User.Login)
		}
		if lbl := github.LabelNames(issue.Labels); lbl != "" {
			fmt.Printf("labels:  %s\n", lbl)
		}
		if issue.HTMLURL != "" {
			fmt.Printf("url:     %s\n", issue.HTMLURL)
		}
		if issue.Body != "" {
			fmt.Printf("\n%s\n", issue.Body)
		}
		for _, c := range cmts {
			who := ""
			if c.User != nil {
				who = c.User.Login
			}
			fmt.Printf("\n--- comment by %s ---\n%s\n", who, c.Body)
		}
	})
}

func issueCreate(args []string) error {
	fs := flag.NewFlagSet("issue create", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		title     = fs.String("title", "", "issue title (required)")
		bodyVal   = fs.String("body", "", "issue body (Markdown)")
		bodyFile  = fs.String("body-file", "", `read body from file ("-" for stdin)`)
		labels    = fs.String("label", "", "comma-separated labels to apply")
		assignees = fs.String("assignee", "", "comma-separated assignee logins")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("title", *title); err != nil {
		return err
	}
	body, err := readBody(*bodyVal, *bodyFile)
	if err != nil {
		return err
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return err
	}
	cl, err := common.client()
	if err != nil {
		return err
	}

	issue, err := cl.CreateIssue(context.Background(), owner, repo, github.CreateIssueInput{
		Title: *title, Body: body, Labels: splitCSV(*labels), Assignees: splitCSV(*assignees),
	})
	if err != nil {
		return err
	}
	return emit(common.output, issue, func() {
		fmt.Printf("created issue #%d\n", issue.Number)
		if issue.HTMLURL != "" {
			fmt.Println(issue.HTMLURL)
		}
	})
}

func issueUpdate(args []string) error {
	fs := flag.NewFlagSet("issue update", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		number   = fs.Int("number", 0, "issue number (required)")
		title    = fs.String("title", "", "new title (optional)")
		bodyVal  = fs.String("body", "", "new body (Markdown)")
		bodyFile = fs.String("body-file", "", `read new body from file ("-" for stdin)`)
		state    = fs.String("state", "", "new state: open|closed")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *number == 0 {
		return fmt.Errorf("--number is required")
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return err
	}
	cl, err := common.client()
	if err != nil {
		return err
	}

	in := github.UpdateIssueInput{Title: *title, State: *state}
	// Only set body when the user actually passed one.
	bodyChanged := isFlagSet(fs, "body") || isFlagSet(fs, "body-file")
	if bodyChanged {
		body, err := readBody(*bodyVal, *bodyFile)
		if err != nil {
			return err
		}
		in.Body = &body
	}

	issue, err := cl.UpdateIssue(context.Background(), owner, repo, *number, in)
	if err != nil {
		return err
	}
	return emit(common.output, issue, func() {
		fmt.Printf("updated issue #%d [%s]\n", issue.Number, issue.State)
		if issue.HTMLURL != "" {
			fmt.Println(issue.HTMLURL)
		}
	})
}

func issueComment(args []string) error {
	fs := flag.NewFlagSet("issue comment", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		number   = fs.Int("number", 0, "issue number (required)")
		bodyVal  = fs.String("body", "", "comment body (Markdown)")
		bodyFile = fs.String("body-file", "", `read body from file ("-" for stdin)`)
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *number == 0 {
		return fmt.Errorf("--number is required")
	}
	body, err := readBody(*bodyVal, *bodyFile)
	if err != nil {
		return err
	}
	if body == "" {
		return fmt.Errorf("comment body is empty (use --body or --body-file)")
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return err
	}
	cl, err := common.client()
	if err != nil {
		return err
	}

	c, err := cl.AddIssueComment(context.Background(), owner, repo, *number, body)
	if err != nil {
		return err
	}
	return emit(common.output, c, func() {
		fmt.Printf("added comment on issue #%d\n", *number)
		if c.HTMLURL != "" {
			fmt.Println(c.HTMLURL)
		}
	})
}

func issueLabel(args []string) error {
	fs := flag.NewFlagSet("issue label", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		number = fs.Int("number", 0, "issue number (required)")
		add    = fs.String("add", "", "comma-separated labels to add")
		remove = fs.String("remove", "", "comma-separated labels to remove")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *number == 0 {
		return fmt.Errorf("--number is required")
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return err
	}
	cl, err := common.client()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, name := range splitCSV(*remove) {
		if err := cl.RemoveIssueLabel(ctx, owner, repo, *number, name); err != nil {
			return fmt.Errorf("removing label %q: %w", name, err)
		}
	}
	if names := splitCSV(*add); len(names) > 0 {
		if _, err := cl.AddIssueLabels(ctx, owner, repo, *number, names); err != nil {
			return fmt.Errorf("adding labels: %w", err)
		}
	}

	issue, err := cl.GetIssue(ctx, owner, repo, *number)
	if err != nil {
		return err
	}
	return emit(common.output, issue.Labels, func() {
		if len(issue.Labels) == 0 {
			fmt.Println("(no labels)")
			return
		}
		fmt.Println(github.LabelNames(issue.Labels))
	})
}

// isFlagSet reports whether a flag was explicitly provided on the command line.
func isFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
