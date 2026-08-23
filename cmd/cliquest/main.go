package main

import (
	"fmt"
	"os"

	"github.com/tensho1026/CLI-Quest/internal/cli"
	"github.com/tensho1026/CLI-Quest/internal/workspace"
)

func main() {
	if handled, err := workspace.RunHelper(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	app, err := cli.New(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	os.Exit(app.Run(os.Args[1:]))
}
