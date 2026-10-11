VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)
COVERAGE_MIN ?= 60
COVERAGE_PROFILE := .build/coverage.out

.PHONY: build check release-check fmt fmt-check module-check test test-race \
	coverage-check vet static-check shell-check compatibility-check \
	python-check documentation-check workflow-check repository-check license-check cask-check smoke-check \
	package-check vuln-check clean

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/njulogin ./cmd/njulogin

fmt:
	gofmt -w cmd/njulogin/*.go

fmt-check:
	test -z "$$(gofmt -l cmd/njulogin/*.go)"

test:
	go test ./...

test-race:
	go test -race ./...

coverage-check:
	mkdir -p .build
	go test -coverprofile="$(COVERAGE_PROFILE)" ./...
	python3 scripts/check-coverage.py "$(COVERAGE_PROFILE)" "$(COVERAGE_MIN)"

vet:
	go vet ./...

static-check:
	staticcheck ./...

shell-check:
	bash -n tools/NJUlogin/njulogin-portal.sh
	bash -n tools/NJUlogin/njulogin-remote.sh
	bash -n scripts/package.sh
	bash -n scripts/render-cask.sh
	bash -n scripts/check-repository-invariants.sh
	bash -n scripts/test-compatibility-tools.sh
	bash -n scripts/test-cask-template.sh
	bash -n scripts/test-package.sh
	shellcheck scripts/package.sh scripts/render-cask.sh \
		scripts/check-repository-invariants.sh scripts/test-compatibility-tools.sh \
		scripts/test-cask-template.sh scripts/test-package.sh \
		tools/NJUlogin/njulogin-portal.sh tools/NJUlogin/njulogin-remote.sh

compatibility-check:
	bash scripts/test-compatibility-tools.sh

python-check:
	python3 -m py_compile scripts/check-coverage.py scripts/check-documentation.py \
		scripts/check-third-party-licenses.py

documentation-check:
	python3 scripts/check-documentation.py

workflow-check:
	actionlint .github/workflows/ci.yml .github/workflows/release.yml

module-check:
	go mod verify
	go mod tidy -diff

repository-check:
	bash scripts/check-repository-invariants.sh

license-check:
	python3 scripts/check-third-party-licenses.py

cask-check:
	bash scripts/test-cask-template.sh

# A semver build checks for updates only when standard error is a terminal,
# so every run sends standard error elsewhere to stay off the network. The
# temporary HOME and XDG_CONFIG_HOME keep the real configuration untouched.
smoke-check: VERSION=0.0.0-check
smoke-check: build
	@set -eu; \
	tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	export HOME="$$tmp/home" XDG_CONFIG_HOME="$$tmp/config"; \
	test "$$(./bin/njulogin version 2>"$$tmp/version-err")" = "njulogin 0.0.0-check"; \
	test "$$(./bin/njulogin --version 2>"$$tmp/version-err")" = "njulogin 0.0.0-check"; \
	./bin/njulogin help 2>&1 | grep -q '^Usage:'; \
	set +e; ./bin/njulogin </dev/null >"$$tmp/stdout" 2>"$$tmp/stderr"; status=$$?; set -e; \
	test "$$status" -eq 2; \
	grep -q '^Usage:' "$$tmp/stderr"; \
	set +e; ./bin/njulogin unknown-command >"$$tmp/unknown-out" 2>"$$tmp/unknown-err"; status=$$?; set -e; \
	test "$$status" -eq 2

vuln-check:
	govulncheck ./...

package-check:
	bash scripts/test-package.sh

check: fmt-check module-check vet test test-race coverage-check static-check \
	shell-check compatibility-check python-check workflow-check \
	documentation-check repository-check license-check cask-check smoke-check

release-check: check package-check vuln-check

clean:
	$(RM) -r bin .build
