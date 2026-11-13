// Command njulogin authenticates to the NJU p.nju.edu.cn portal, locally or
// through an SSH host.
package main

import (
	"os"

	"github.com/soundadam/njulogin/internal/cli"
)

// version is set by the release build with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Main(version, os.Args[1:]))
}
