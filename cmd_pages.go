package main

import (
	"context"
	"flag"
	"fmt"

	"github-docs-cli/internal/github"
)

func runPages(args []string) error {
	usage := subUsage("pages", "Manage GitHub Pages publishing for the repository.", []string{
		"get     Show the current Pages configuration/status",
		"enable  Configure Pages to build from a branch/path (--branch, --path)",
		"build   Trigger a new Pages build",
	})
	return dispatch("pages", args, usage, map[string]func([]string) error{
		"get":    pagesGet,
		"enable": pagesEnable,
		"build":  pagesBuild,
	})
}

func pagesGet(args []string) error {
	fs := flag.NewFlagSet("pages get", flag.ExitOnError)
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

	info, err := cl.GetPages(context.Background(), owner, repo)
	if err != nil {
		if github.NotFound(err) {
			return fmt.Errorf("GitHub Pages is not enabled for %s/%s (use: pages enable)", owner, repo)
		}
		return err
	}
	return emit(common.output, info, func() {
		fmt.Printf("status:  %s\n", info.Status)
		fmt.Printf("url:     %s\n", info.HTMLURL)
		if info.Source != nil {
			fmt.Printf("source:  %s /%s\n", info.Source.Branch, info.Source.Path)
		}
	})
}

func pagesEnable(args []string) error {
	fs := flag.NewFlagSet("pages enable", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		branch = fs.String("branch", "", "source branch (default: repository default branch)")
		path   = fs.String("path", "/docs", "source path: / or /docs")
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
	ctx := context.Background()

	br := *branch
	if br == "" {
		br, err = cl.DefaultBranch(ctx, owner, repo)
		if err != nil {
			return err
		}
	}

	info, err := cl.EnablePages(ctx, owner, repo, br, *path)
	if err != nil {
		return err
	}
	return emit(common.output, info, func() {
		fmt.Printf("Pages enabled from %s %s\n", br, *path)
		if info.HTMLURL != "" {
			fmt.Println(info.HTMLURL)
		}
	})
}

func pagesBuild(args []string) error {
	fs := flag.NewFlagSet("pages build", flag.ExitOnError)
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

	if err := cl.RequestPagesBuild(context.Background(), owner, repo); err != nil {
		return err
	}
	result := map[string]string{"repo": owner + "/" + repo, "status": "build requested"}
	return emit(common.output, result, func() {
		fmt.Println("Pages build requested")
	})
}
