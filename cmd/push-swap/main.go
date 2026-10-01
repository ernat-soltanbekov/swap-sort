package main

import (
	"os"

	"github.com/ernat-soltanbekov/swap-sort/internal/cli"
)

func main() { os.Exit(cli.PushSwap(os.Args[1:], os.Stdout, os.Stderr)) }
