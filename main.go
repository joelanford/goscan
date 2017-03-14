package main

import (
	"fmt"
	"os"

	"github.com/joelanford/goscan/app"
	"github.com/joelanford/goscan/app/cli"
)

func main() {
	opts, err := app.ParseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cli.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
