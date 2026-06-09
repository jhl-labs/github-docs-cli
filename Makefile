BINARY  := github-docs-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Platforms for `make dist`.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: build test vet fmt clean dist install skills

# Flavors rendered for the docs/ skill preview.
SKILL_FLAVORS := generic claude codex gemini opencode

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

install:
	go install -ldflags "$(LDFLAGS)" .

# Cross-compile release binaries into dist/ with stable, version-less names
# (so https://.../releases/latest/download/<asset> links stay valid), plus
# a SHA256SUMS file for verification.
dist:
	@rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/$(BINARY)-$$os-$$arch$$ext"; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o $$out . || exit 1; \
	done
	@cd dist && (sha256sum * > SHA256SUMS 2>/dev/null || shasum -a 256 * > SHA256SUMS)
	@echo "dist/ contents:" && ls -1 dist

# Regenerate the skill markdown files previewed on the docs/ landing page.
# Run this whenever the generate-skill output changes so the site stays in sync.
skills: build
	@mkdir -p docs/skills
	@for fl in $(SKILL_FLAVORS); do \
		arg=$$fl; [ "$$fl" = "generic" ] && arg=""; \
		./$(BINARY) generate-skill $$arg --out "docs/skills/github-docs-skill-$$fl.md" --force; \
	done

clean:
	rm -rf $(BINARY) $(BINARY).exe dist/
