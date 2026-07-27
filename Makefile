VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build check fmt fmt-check test vet shell-check python-check clean

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/njulogin ./cmd/njulogin

fmt:
	gofmt -w cmd/njulogin/*.go

fmt-check:
	test -z "$$(gofmt -l cmd/njulogin/*.go)"

test:
	go test ./...

vet:
	go vet ./...

shell-check:
	bash -n tools/NJUlogin/njulogin-portal.sh
	bash -n tools/NJUlogin/njulogin-remote.sh
	bash -n scripts/render-formula.sh

python-check:
	python3 -m py_compile cmd/njulogin/remote_qr_bootstrap.py

check: fmt-check vet test shell-check python-check

clean:
	$(RM) -r bin
