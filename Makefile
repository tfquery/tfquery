.PHONY: default build check clean docs install lint release release-check shadow sign-test static test tflint vet

CLEAN_DAYS=30
INSTALL_DIR=${HOME}/bin
OUT=/tmp/tfquery

define RELEASE_SHARED_CHECKS
	@if ! echo "$(VERSION)" | grep --extended-regexp --quiet '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "=== ERROR: VERSION must be a valid semantic version (e.g. v0.2.0) with leading 'v'. Got: $(VERSION)"; \
		exit 1; \
	fi

	@if ! grep --extended-regexp --quiet "^\#\# $(VERSION)" CHANGELOG.md; then \
		echo "=== ERROR: CHANGELOG.md entry missing for $(VERSION)."; \
		exit 1; \
	fi

	@if [ -n "$$(git tag --list '$(VERSION)')" ]; then \
		echo "=== ERROR: Local tag $(VERSION) already exists."; \
		exit 1; \
	fi

	@if git ls-remote --tags origin "$(VERSION)" | grep --quiet .; then \
		echo "=== ERROR: Remote tag $(VERSION) already exists."; \
		exit 1; \
	fi

	@if ! command -v gh >/dev/null 2>&1; then \
		echo "=== ERROR: GitHub CLI (gh) is required."; \
		exit 1; \
	fi

	@if gh release view "$(VERSION)" --repo tfquery/tfquery >/dev/null 2>&1; then \
		echo "=== ERROR: GitHub release $(VERSION) already exists."; \
		exit 1; \
	fi
endef

default: build


BUILDVER := $(shell git describe --always --dirty --long --tags)
build:
	go build \
		-ldflags="-X github.com/tfquery/tfquery/internal/version.Version=$(BUILDVER)" \
		-trimpath \
		-o $(OUT)


check:
	@status=0; \
	for target in lint shadow static vet test; do \
		$(MAKE) $$target || status=1; \
	done; \
	exit $$status

clean:
	tools/clean.sh 30

docs: recast
	@if [ -z "$(VERSION)" ]; then echo "Usage: make docs VERSION=x.y.z"; exit 1; fi
	@version="$(VERSION)"; version="$${version}"; go run ./tools/docsgen/main.go ./docs "$$version"

install: build
	mv $(OUT) $(INSTALL_DIR)

lint:
	golangci-lint run

recast:
	@set -e; \
	for cast in docs/asciinema/*.cast; do \
		[ -e "$$cast" ] || continue; \
		gif="$${cast%.cast}.gif"; \
		if [ ! -e "$$gif" ] || [ "$$cast" -nt "$$gif" ]; then \
			echo "=== Rendering $$cast"; \
			agg --verbose "$$cast" "$$gif" --fps-cap 24; \
		else \
			echo "=== Skipping $$cast"; \
		fi; \
	done

release:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=x.y.z"; exit 1; fi

	$(RELEASE_SHARED_CHECKS)

	@$(MAKE) docs VERSION="$(VERSION)"

	@if [ -n "$$(git status --porcelain -- docs/)" ]; then \
		echo "=== Docs changed after generation; committing docs updates."; \
		git add docs/; \
		git commit --message "docs: regenerate docs for $(VERSION)."; \
	fi

	git push origin --delete "$(VERSION)" || true
	git tag --delete "$(VERSION)" || true
	git tag "$(VERSION)" --message "Release $(VERSION)."
	git push origin HEAD
	git push origin "$(VERSION)"

release-check:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release-check VERSION=vx.y.z"; exit 1; fi

	$(RELEASE_SHARED_CHECKS)

	@awk -v tag="$(VERSION)" '\
		BEGIN { in_section=0; found=0; n=0 } \
		{ sub(/\r$$/, "") } \
		$$0 ~ ("^## " tag "([[:space:]]+-[[:space:]].*)?$$") { in_section=1; found=1; print; next } \
		in_section && $$0 ~ "^## v[0-9]+\\.[0-9]+\\.[0-9]+([-.].*)?([[:space:]]+-[[:space:]].*)?$$" { exit } \
		in_section { print; n++ } \
		END { \
			if (!found) { print "ERROR: release heading not found for " tag > "/dev/stderr"; exit 1 } \
			if (n == 0) { print "ERROR: release notes section empty for " tag > "/dev/stderr"; exit 1 } \
		} \
	' CHANGELOG.md > /tmp/release-notes.md

	@goreleaser check
	@echo "=== release-check passed"

sign-test:
	@set -eu; \
	if ! command -v gpg >/dev/null 2>&1; then \
		echo "=== ERROR: gpg is required."; \
		exit 1; \
	fi; \
	if ! gpg --list-secret-keys --with-colons | grep -q '^sec'; then \
		echo "=== ERROR: No private GPG key is available for signing."; \
		echo "=== Import your release key first, then retry."; \
		exit 1; \
	fi; \
	tmpdir="$$(mktemp -d /tmp/tfquery-sign-test.XXXXXX)"; \
	cleanup() { rm -rf "$$tmpdir"; }; \
	trap cleanup EXIT INT TERM; \
	artifact="$$tmpdir/tfquery-sign-test.txt"; \
	signature="$$artifact.sig"; \
	verify_home="$$tmpdir/verify-gnupg"; \
	printf 'tfquery sign-test generated at %s\n' "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$$artifact"; \
	gpg --batch --yes --armor --detach-sign --output "$$signature" "$$artifact"; \
	gpg --batch --verify "$$signature" "$$artifact" >/dev/null 2>&1; \
	mkdir -m 700 -p "$$verify_home"; \
	GNUPGHOME="$$verify_home" gpg --batch --import KEYS >/dev/null 2>&1; \
	if ! GNUPGHOME="$$verify_home" gpg --batch --verify "$$signature" "$$artifact" >/dev/null 2>&1; then \
		echo "=== ERROR: Signature verifies in your default keyring but not with KEYS."; \
		echo "=== Ensure your signing key matches the public key in KEYS."; \
		exit 1; \
	fi; \
	echo "=== sign-test passed"

shadow:
	shadow ./...

static:
	staticcheck ./...

test:
	go test ./... --count 1 -v

vet:
	go vet -printfuncs='Debugf,Infof,Warnf,Errorf' ./...
