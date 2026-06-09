package main

import (
	"context"
	"flag"
	"fmt"
)

func runDiscussion(args []string) error {
	usage := subUsage("discussion", "Manage GitHub Discussions (via the GraphQL API).", []string{
		"list        List recent discussions (--limit)",
		"get         Show a discussion with comments (--number)",
		"create      Open a discussion (--category, --title, --body/--body-file)",
		"comment     Add a comment (--number, --body/--body-file)",
		"categories  List the repository's discussion categories",
	})
	return dispatch("discussion", args, usage, map[string]func([]string) error{
		"list":       discussionList,
		"get":        discussionGet,
		"create":     discussionCreate,
		"comment":    discussionComment,
		"categories": discussionCategories,
	})
}

func discussionList(args []string) error {
	fs := flag.NewFlagSet("discussion list", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	limit := fs.Int("limit", 25, "maximum discussions to return")
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

	ds, err := cl.ListDiscussions(context.Background(), owner, repo, *limit)
	if err != nil {
		return err
	}
	return emit(common.output, ds, func() {
		if len(ds) == 0 {
			fmt.Println("no discussions")
			return
		}
		for _, d := range ds {
			fmt.Printf("#%-5d [%s] %s\n", d.Number, d.Category.Name, d.Title)
		}
		fmt.Printf("\n%d discussion(s)\n", len(ds))
	})
}

func discussionGet(args []string) error {
	fs := flag.NewFlagSet("discussion get", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	number := fs.Int("number", 0, "discussion number (required)")
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

	d, err := cl.GetDiscussion(context.Background(), owner, repo, *number)
	if err != nil {
		return err
	}
	return emit(common.output, d, func() {
		fmt.Printf("#%d [%s] %s\n", d.Number, d.Category.Name, d.Title)
		if d.Author != nil {
			fmt.Printf("author:  %s\n", d.Author.Login)
		}
		if d.URL != "" {
			fmt.Printf("url:     %s\n", d.URL)
		}
		if d.Body != "" {
			fmt.Printf("\n%s\n", d.Body)
		}
		for _, c := range d.Comments {
			who := ""
			if c.Author != nil {
				who = c.Author.Login
			}
			fmt.Printf("\n--- comment by %s ---\n%s\n", who, c.Body)
		}
	})
}

func discussionCreate(args []string) error {
	fs := flag.NewFlagSet("discussion create", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		category = fs.String("category", "", "discussion category name (required)")
		title    = fs.String("title", "", "discussion title (required)")
		bodyVal  = fs.String("body", "", "discussion body (Markdown)")
		bodyFile = fs.String("body-file", "", `read body from file ("-" for stdin)`)
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("category", *category); err != nil {
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

	d, err := cl.CreateDiscussion(context.Background(), owner, repo, *category, *title, body)
	if err != nil {
		return err
	}
	return emit(common.output, d, func() {
		fmt.Printf("created discussion #%d\n", d.Number)
		if d.URL != "" {
			fmt.Println(d.URL)
		}
	})
}

func discussionComment(args []string) error {
	fs := flag.NewFlagSet("discussion comment", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		number   = fs.Int("number", 0, "discussion number (required)")
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

	c, err := cl.AddDiscussionComment(context.Background(), owner, repo, *number, body)
	if err != nil {
		return err
	}
	return emit(common.output, c, func() {
		fmt.Printf("added comment on discussion #%d\n", *number)
		if c.URL != "" {
			fmt.Println(c.URL)
		}
	})
}

func discussionCategories(args []string) error {
	fs := flag.NewFlagSet("discussion categories", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
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

	cats, err := cl.ListDiscussionCategories(context.Background(), owner, repo)
	if err != nil {
		return err
	}
	return emit(common.output, cats, func() {
		if len(cats) == 0 {
			fmt.Println("no categories")
			return
		}
		for _, c := range cats {
			fmt.Printf("%-20s %s\n", c.Name, c.Description)
		}
	})
}
