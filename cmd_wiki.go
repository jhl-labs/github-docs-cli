package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github-docs-cli/internal/config"
)

// The GitHub wiki is a separate git repository (`<repo>.wiki.git`). There is no
// REST API for it, so these commands clone it over HTTPS, mutate it locally,
// and push. They require the `git` binary on PATH.

func runWiki(args []string) error {
	usage := subUsage("wiki", "Manage the repository wiki (separate git repo; requires the git binary).", []string{
		"list    List wiki pages",
		"get     Print a wiki page (--page)",
		"set     Create/update a wiki page (--page, --body/--body-file, --message)",
		"delete  Delete a wiki page (--page, --message)",
	})
	return dispatch("wiki", args, usage, map[string]func([]string) error{
		"list":   wikiList,
		"get":    wikiGet,
		"set":    wikiSet,
		"delete": wikiDelete,
	})
}

// wikiContext carries everything needed to operate on a cloned wiki.
type wikiContext struct {
	dir   string // local clone directory
	clean func()
}

// cloneWiki shallow-clones the wiki repo into a temp dir.
func cloneWiki(common *commonFlags) (*wikiContext, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git is required for wiki commands but was not found on PATH")
	}
	owner, repo, err := common.ownerRepo()
	if err != nil {
		return nil, err
	}
	if common.token == "" {
		return nil, fmt.Errorf("a token is required to access the wiki (--token / %s)", config.EnvToken)
	}
	host := gitHost(common.apiURL)
	cloneURL := fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.wiki.git", common.token, host, owner, repo)

	dir, err := os.MkdirTemp("", "ghdocs-wiki-")
	if err != nil {
		return nil, err
	}
	clean := func() { os.RemoveAll(dir) }

	if out, err := runGit("", "clone", "--depth", "1", cloneURL, dir); err != nil {
		clean()
		return nil, fmt.Errorf("cloning wiki (is the wiki initialized?): %v: %s", err, out)
	}
	return &wikiContext{dir: dir, clean: clean}, nil
}

func wikiList(args []string) error {
	fs := flag.NewFlagSet("wiki list", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	wc, err := cloneWiki(common)
	if err != nil {
		return err
	}
	defer wc.clean()

	pages, err := listWikiPages(wc.dir)
	if err != nil {
		return err
	}
	return emit(common.output, pages, func() {
		if len(pages) == 0 {
			fmt.Println("no wiki pages")
			return
		}
		for _, p := range pages {
			fmt.Println(p)
		}
	})
}

func wikiGet(args []string) error {
	fs := flag.NewFlagSet("wiki get", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	page := fs.String("page", "", "wiki page name (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("page", *page); err != nil {
		return err
	}

	wc, err := cloneWiki(common)
	if err != nil {
		return err
	}
	defer wc.clean()

	file := filepath.Join(wc.dir, wikiFilename(*page))
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("wiki page %q not found", *page)
	}
	if common.output == "json" {
		return emit("json", map[string]string{"page": *page, "content": string(data)}, nil)
	}
	fmt.Print(string(data))
	return nil
}

func wikiSet(args []string) error {
	fs := flag.NewFlagSet("wiki set", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		page     = fs.String("page", "", "wiki page name (required)")
		bodyVal  = fs.String("body", "", "page content (Markdown)")
		bodyFile = fs.String("body-file", "", `read content from file ("-" for stdin)`)
		message  = fs.String("message", "", "commit message (default: generated)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("page", *page); err != nil {
		return err
	}
	content, err := readBody(*bodyVal, *bodyFile)
	if err != nil {
		return err
	}

	wc, err := cloneWiki(common)
	if err != nil {
		return err
	}
	defer wc.clean()

	name := wikiFilename(*page)
	if err := os.WriteFile(filepath.Join(wc.dir, name), []byte(content), 0o644); err != nil {
		return err
	}
	msg := *message
	if msg == "" {
		msg = "Update " + *page
	}
	if err := commitAndPush(wc.dir, name, msg); err != nil {
		return err
	}
	return emit(common.output, map[string]string{"page": *page, "status": "saved"}, func() {
		fmt.Printf("saved wiki page %q\n", *page)
	})
}

func wikiDelete(args []string) error {
	fs := flag.NewFlagSet("wiki delete", flag.ExitOnError)
	common, err := registerCommon(fs)
	if err != nil {
		return err
	}
	var (
		page    = fs.String("page", "", "wiki page name (required)")
		message = fs.String("message", "", "commit message (default: generated)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireFlag("page", *page); err != nil {
		return err
	}

	wc, err := cloneWiki(common)
	if err != nil {
		return err
	}
	defer wc.clean()

	name := wikiFilename(*page)
	if _, err := os.Stat(filepath.Join(wc.dir, name)); err != nil {
		return fmt.Errorf("wiki page %q not found", *page)
	}
	if out, err := runGit(wc.dir, "rm", name); err != nil {
		return fmt.Errorf("git rm: %v: %s", err, out)
	}
	msg := *message
	if msg == "" {
		msg = "Delete " + *page
	}
	if out, err := runGit(wc.dir, "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %v: %s", err, out)
	}
	if out, err := runGit(wc.dir, "push"); err != nil {
		return fmt.Errorf("git push: %v: %s", err, out)
	}
	return emit(common.output, map[string]string{"page": *page, "status": "deleted"}, func() {
		fmt.Printf("deleted wiki page %q\n", *page)
	})
}

// listWikiPages returns the page names (without the .md extension) in a clone.
func listWikiPages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var pages []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") {
			pages = append(pages, wikiPageName(name))
		}
	}
	sort.Strings(pages)
	return pages, nil
}

// wikiFilename maps a page name to its on-disk filename ("My Page" -> "My-Page.md").
func wikiFilename(page string) string {
	page = strings.TrimSuffix(page, ".md")
	page = strings.ReplaceAll(page, " ", "-")
	return page + ".md"
}

// wikiPageName is the inverse of wikiFilename for display.
func wikiPageName(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	return strings.ReplaceAll(name, "-", " ")
}

// commitAndPush stages one file, commits, and pushes.
func commitAndPush(dir, file, message string) error {
	if out, err := runGit(dir, "add", file); err != nil {
		return fmt.Errorf("git add: %v: %s", err, out)
	}
	// Nothing to commit is not an error worth failing on.
	if out, err := runGit(dir, "commit", "-m", message); err != nil {
		if strings.Contains(out, "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %v: %s", err, out)
	}
	if out, err := runGit(dir, "push"); err != nil {
		return fmt.Errorf("git push: %v: %s", err, out)
	}
	return nil
}

// runGit runs a git command in dir (or the current dir when dir is "").
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Identify commits even when no global git identity is configured.
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=github-docs-cli",
		"GIT_AUTHOR_EMAIL=github-docs-cli@users.noreply.github.com",
		"GIT_COMMITTER_NAME=github-docs-cli",
		"GIT_COMMITTER_EMAIL=github-docs-cli@users.noreply.github.com",
		"GIT_TERMINAL_PROMPT=0",
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// gitHost derives the git web host from the REST API base URL.
//
//	https://api.github.com         -> github.com
//	https://ghe.example.com/api/v3 -> ghe.example.com
func gitHost(apiURL string) string {
	h := strings.TrimRight(apiURL, "/")
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if h == "api.github.com" {
		return "github.com"
	}
	h = strings.TrimSuffix(h, "/api/v3")
	return h
}
