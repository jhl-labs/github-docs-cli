package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github-docs-cli/internal/github"
)

// noteKinds are the recognized note classifications.
var noteKinds = map[string]bool{"dev": true, "research": true, "decision": true, "note": true}

func runNote(args []string) error {
	usage := subUsage("note", "Record a dev/research/decision note to the repo — designed for AI-agent hooks.", []string{
		"add  Append/post a note. Target one of:",
		"       --docs PATH       append to a Markdown file (default: docs/notes/DEVLOG.md)",
		"       --issue N         add as a comment on issue N",
		"       --new-issue       open a new issue labelled note:<kind>",
		"       --discussion N    add as a comment on discussion N",
		"     Common flags: --kind dev|research|decision, --title, --body/--body-file",
	})
	return dispatch("note", args, usage, map[string]func([]string) error{
		"add": noteAdd,
	})
}

func noteAdd(args []string) error {
	fs := flag.NewFlagSet("note add", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		kind       = fs.String("kind", "note", "note kind: dev|research|decision|note")
		title      = fs.String("title", "", "short note title")
		bodyVal    = fs.String("body", "", "note body (Markdown)")
		bodyFile   = fs.String("body-file", "", `read body from file ("-" for stdin)`)
		docs       = fs.String("docs", "", "append to this Markdown file path")
		issue      = fs.Int("issue", 0, "post as a comment on this issue number")
		newIssue   = fs.Bool("new-issue", false, "open a new issue for the note")
		discussion = fs.Int("discussion", 0, "post as a comment on this discussion number")
		message    = fs.String("message", "", "commit message for --docs (default: generated)")
		branch     = fs.String("branch", "", "branch for --docs (default: default branch)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !noteKinds[*kind] {
		return fmt.Errorf("unknown kind %q (use dev|research|decision|note)", *kind)
	}
	body, err := readBody(*bodyVal, *bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" && strings.TrimSpace(*title) == "" {
		return fmt.Errorf("provide at least --title or --body")
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

	timestamp := time.Now().UTC().Format(time.RFC3339)
	entry := formatNoteEntry(*kind, *title, body, timestamp)

	switch {
	case *issue > 0:
		c, err := cl.AddIssueComment(ctx, owner, repo, *issue, entry)
		if err != nil {
			return err
		}
		return emit(common.output, c, func() {
			fmt.Printf("recorded %s note on issue #%d\n", *kind, *issue)
			if c.HTMLURL != "" {
				fmt.Println(c.HTMLURL)
			}
		})

	case *discussion > 0:
		c, err := cl.AddDiscussionComment(ctx, owner, repo, *discussion, entry)
		if err != nil {
			return err
		}
		return emit(common.output, c, func() {
			fmt.Printf("recorded %s note on discussion #%d\n", *kind, *discussion)
			if c.URL != "" {
				fmt.Println(c.URL)
			}
		})

	case *newIssue:
		t := *title
		if t == "" {
			t = fmt.Sprintf("[%s] note %s", *kind, timestamp)
		} else {
			t = fmt.Sprintf("[%s] %s", *kind, t)
		}
		iss, err := cl.CreateIssue(ctx, owner, repo, github.CreateIssueInput{
			Title:  t,
			Body:   body,
			Labels: []string{"note", "note:" + *kind},
		})
		if err != nil {
			return err
		}
		return emit(common.output, iss, func() {
			fmt.Printf("recorded %s note as issue #%d\n", *kind, iss.Number)
			if iss.HTMLURL != "" {
				fmt.Println(iss.HTMLURL)
			}
		})

	default:
		// Default destination: append to a Markdown log file in the repo.
		path := *docs
		if path == "" {
			path = "docs/notes/DEVLOG.md"
		}
		commit, err := appendToDocs(ctx, cl, owner, repo, path, entry, *message, *branch, *kind)
		if err != nil {
			return err
		}
		return emit(common.output, commit, func() {
			fmt.Printf("recorded %s note in %s\n", *kind, path)
			if commit.Commit.HTMLURL != "" {
				fmt.Println(commit.Commit.HTMLURL)
			}
		})
	}
}

// appendToDocs appends an entry to a Markdown file, creating it if missing.
func appendToDocs(ctx context.Context, cl *github.Client, owner, repo, path, entry, message, branch, kind string) (*github.FileCommit, error) {
	var (
		existing string
		sha      string
	)
	if file, err := cl.GetFile(ctx, owner, repo, path, branch); err == nil {
		existing, err = file.Decoded()
		if err != nil {
			return nil, err
		}
		sha = file.SHA
	} else if !github.NotFound(err) {
		return nil, err
	}

	var b strings.Builder
	if existing == "" {
		b.WriteString("# Notes\n\n")
	} else {
		b.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString(entry)

	msg := message
	if msg == "" {
		msg = fmt.Sprintf("note(%s): record entry", kind)
	}
	return cl.PutFile(ctx, owner, repo, github.PutFileInput{
		Path: path, Content: b.String(), Message: msg, SHA: sha, Branch: branch,
	})
}

// formatNoteEntry renders a single timestamped Markdown note section.
func formatNoteEntry(kind, title, body, timestamp string) string {
	heading := title
	if heading == "" {
		heading = "(untitled)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## [%s] %s — %s\n\n", kind, timestamp, heading)
	if strings.TrimSpace(body) != "" {
		b.WriteString(strings.TrimRight(body, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}
