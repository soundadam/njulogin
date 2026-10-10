VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)
COVERAGE_MIN ?= 60
COVERAGE_PROFILE := .build/coverage.out

.PHONY: build check release-check fmt fmt-check module-check test test-race \
	coverage-check vet static-check shell-check compatibility-check \
	python-check documentation-check workflow-check repository-check license-check formula-check smoke-check \
	cross-build-check vuln-check clean

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
	bash -n scripts/render-formula.sh
	bash -n scripts/check-repository-invariants.sh
	bash -n scripts/test-compatibility-tools.sh
	bash -n scripts/test-formula-template.sh
	shellcheck scripts/render-formula.sh \
		scripts/check-repository-invariants.sh scripts/test-compatibility-tools.sh \
		scripts/test-formula-template.sh \
		tools/NJUlogin/njulogin-portal.sh tools/NJUlogin/njulogin-remote.sh

compatibility-check:
	bash scripts/test-compatibility-tools.sh

python-check:
	python3 -m py_compile scripts/check-coverage.py scripts/check-documentation.py \
		scripts/check-third-party-licenses.py

documentation-check:
	python3 scripts/check-documentation.py

workflow-check:
	actionlint .github/workflows/ci.yml

module-check:
	go mod verify
	go mod tidy -diff

repository-check:
	bash scripts/check-repository-invariants.sh

license-check:
	python3 scripts/check-third-party-licenses.py

formula-check:
	bash scripts/test-formula-template.sh

smoke-check: VERSION=0.0.0-check
smoke-check: build
	@set -eu; \
	test "$$(./bin/njulogin version)" = "njulogin 0.0.0-check"; \
	./bin/njulogin help 2>&1 | grep -q '^Usage:'; \
	tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	set +e; ./bin/njulogin </dev/null >"$$tmp/stdout" 2>"$$tmp/stderr"; status=$$?; set -e; \
	test "$$status" -eq 2; \
	grep -q '^Usage:' "$$tmp/stderr"; \
	set +e; ./bin/njulogin unknown-command >"$$tmp/unknown-out" 2>"$$tmp/unknown-err"; status=$$?; set -e; \
	test "$$status" -eq 2

vuln-check:
	govulncheck ./...

cross-build-check:
	mkdir -p .build/cross
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -o .build/cross/njulogin-darwin-amd64 ./cmd/njulogin
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -o .build/cross/njulogin-darwin-arm64 ./cmd/njulogin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o .build/cross/njulogin-linux-amd64 ./cmd/njulogin
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o .build/cross/njulogin-linux-arm64 ./cmd/njulogin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o .build/cross/njulogin-windows-amd64.exe ./cmd/njulogin

check: fmt-check module-check vet test test-race coverage-check static-check \
	shell-check compatibility-check python-check workflow-check \
	documentation-check repository-check license-check formula-check smoke-check

release-check: check cross-build-check vuln-check

clean:
	$(RM) -r bin .build
