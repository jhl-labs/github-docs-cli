package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github-docs-cli/internal/github"
)

func runDocs(args []string) error {
	usage := subUsage("docs", "Manage files in the repository (e.g. the docs/ folder for GitHub Pages).", []string{
		"list    List a directory (--path, default docs)",
		"tree    Print a recursive file tree (--path, --ref)",
		"get     Print a file's contents (--path)",
		"put     Create or update a file (--path, --body/--body-file, --message)",
		"delete  Delete a file (--path, --message)",
	})
	return dispatch("docs", args, usage, map[string]func([]string) error{
		"list":   docsList,
		"tree":   docsTree,
		"get":    docsGet,
		"put":    docsPut,
		"delete": docsDelete,
	})
}

func docsList(args []string) error {
	fs := flag.NewFlagSet("docs list", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		path = fs.String("path", "docs", "directory path to list")
		ref  = fs.String("ref", "", "branch, tag, or commit (default: default branch)")
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

	entries, err := cl.ListDir(context.Background(), owner, repo, *path, *ref)
	if err != nil {
		return err
	}
	return emit(common.output, entries, func() {
		if len(entries) == 0 {
			fmt.Println("(empty)")
			return
		}
		for _, e := range entries {
			marker := " "
			if e.Type == "dir" {
				marker = "/"
			}
			fmt.Printf("%-4s %s%s\n", e.Type, e.Path, marker)
		}
	})
}

func docsTree(args []string) error {
	fs := flag.NewFlagSet("docs tree", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		path = fs.String("path", "docs", "subtree path prefix to show (empty for whole repo)")
		ref  = fs.String("ref", "", "branch, tag, or commit (default: default branch)")
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

	branch := *ref
	if branch == "" {
		branch, err = cl.DefaultBranch(ctx, owner, repo)
		if err != nil {
			return err
		}
	}
	tree, err := cl.GetTree(ctx, owner, repo, branch, true)
	if err != nil {
		return err
	}

	prefix := strings.Trim(*path, "/")
	paths := make([]string, 0, len(tree.Tree))
	for _, e := range tree.Tree {
		if e.Type != "blob" {
			continue
		}
		if prefix == "" || e.Path == prefix || strings.HasPrefix(e.Path, prefix+"/") {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)

	return emit(common.output, paths, func() {
		if len(paths) == 0 {
			fmt.Println("(no files)")
			return
		}
		printPathTree(paths)
		if tree.Truncated {
			fmt.Println("… (tree truncated by the API)")
		}
	})
}

func docsGet(args []string) error {
	fs := flag.NewFlagSet("docs get", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		path = fs.String("path", "", "file path (required)")
		ref  = fs.String("ref", "", "branch, tag, or commit (default: default branch)")
		meta = fs.Bool("meta", false, "print metadata instead of the raw file (text output)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("path", *path); err != nil {
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

	file, err := cl.GetFile(context.Background(), owner, repo, *path, *ref)
	if err != nil {
		return err
	}
	content, err := file.Decoded()
	if err != nil {
		return err
	}

	if common.output == "json" {
		// Return decoded content for easy consumption.
		return emit(common.output, map[string]any{
			"path": file.Path, "sha": file.SHA, "size": file.Size, "content": content,
		}, nil)
	}
	if *meta {
		fmt.Printf("path: %s\nsha:  %s\nsize: %d\n", file.Path, file.SHA, file.Size)
		return nil
	}
	fmt.Print(content)
	return nil
}

func docsPut(args []string) error {
	fs := flag.NewFlagSet("docs put", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		path     = fs.String("path", "", "file path (required)")
		bodyVal  = fs.String("body", "", "file content")
		bodyFile = fs.String("body-file", "", `read content from file ("-" for stdin)`)
		message  = fs.String("message", "", "commit message (required)")
		branch   = fs.String("branch", "", "target branch (default: default branch)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("path", *path); err != nil {
		return err
	}
	if err := requireFlag("message", *message); err != nil {
		return err
	}
	content, err := readBody(*bodyVal, *bodyFile)
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
	ctx := context.Background()

	// Look up the current blob SHA so an existing file is updated, not rejected.
	sha := ""
	if existing, err := cl.GetFile(ctx, owner, repo, *path, *branch); err == nil {
		sha = existing.SHA
	} else if !github.NotFound(err) {
		return err
	}

	commit, err := cl.PutFile(ctx, owner, repo, github.PutFileInput{
		Path: *path, Content: content, Message: *message, SHA: sha, Branch: *branch,
	})
	if err != nil {
		return err
	}
	return emit(common.output, commit, func() {
		action := "created"
		if sha != "" {
			action = "updated"
		}
		fmt.Printf("%s %s\n", action, *path)
		if commit.Commit.HTMLURL != "" {
			fmt.Println(commit.Commit.HTMLURL)
		}
	})
}

func docsDelete(args []string) error {
	fs := flag.NewFlagSet("docs delete", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		path    = fs.String("path", "", "file path (required)")
		message = fs.String("message", "", "commit message (required)")
		branch  = fs.String("branch", "", "target branch (default: default branch)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("path", *path); err != nil {
		return err
	}
	if err := requireFlag("message", *message); err != nil {
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

	existing, err := cl.GetFile(ctx, owner, repo, *path, *branch)
	if err != nil {
		return err
	}
	commit, err := cl.DeleteFile(ctx, owner, repo, *path, *message, existing.SHA, *branch)
	if err != nil {
		return err
	}
	return emit(common.output, commit, func() {
		fmt.Printf("deleted %s\n", *path)
	})
}

// printPathTree renders a sorted list of slash-separated paths as an indented
// tree.
func printPathTree(paths []string) {
	seen := map[string]bool{}
	for _, p := range paths {
		parts := strings.Split(p, "/")
		for i := range parts {
			branch := strings.Join(parts[:i+1], "/")
			if seen[branch] {
				continue
			}
			seen[branch] = true
			indent := strings.Repeat("  ", i)
			name := parts[i]
			if i < len(parts)-1 {
				name += "/"
			}
			fmt.Printf("%s%s\n", indent, name)
		}
	}
}
