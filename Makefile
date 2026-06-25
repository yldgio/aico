.PHONY: test vet build release

# ── Development ──────────────────────────────────────────────────────────────

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o aico .

check: vet test build

# ── Release ──────────────────────────────────────────────────────────────────
# Usage:
#   make release VERSION=0.4.0     # explicit version
#   make release                   # auto-detect from conventional commits
#
# This generates the changelog, commits, and creates a git tag.
# It does NOT push — review the diff first, then:
#   git push origin main --tags

release:
	@command -v git-cliff >/dev/null 2>&1 || { echo "error: git-cliff not found. Install: https://git-cliff.org/docs/installation"; exit 1; }
	@if [ -z "$(VERSION)" ]; then \
		VERSION=$$(git-cliff --bumped-version | sed 's/^v//'); \
		if [ -z "$$VERSION" ] || [ "$$VERSION" = "$$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')" ]; then \
			echo "error: no conventional commits since last tag — nothing to release."; \
			echo "fix: make a feat/fix commit first, or specify VERSION=x.y.z explicitly."; \
			exit 1; \
		fi; \
		echo "auto-detected version: $$VERSION"; \
	else \
		VERSION=$(VERSION); \
		echo "explicit version: $$VERSION"; \
	fi; \
	git-cliff --tag "v$$VERSION" -o CHANGELOG.md && \
	git add CHANGELOG.md cliff.toml && \
	git commit -m "docs: promote changelog [$$VERSION]" && \
	git tag -a "v$$VERSION" -m "aico v$$VERSION" && \
	echo "" && \
	echo "✅ Tagged v$$VERSION. Review the commit, then push:" && \
	echo "   git push origin main --tags"
