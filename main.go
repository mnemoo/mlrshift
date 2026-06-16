// Command mlrshift surgically shifts a LUT's Most-Likely-Result frequency by
// reweighting a headerless id,weight,payout CSV. See `mlrshift help`.
package main

import (
	"os"

	"github.com/mnemoo/mlrshift/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
