package main

import (
	"os"

	"github.com/joelanford/goscan/utils/unar"
)

func main() {
	unar.Run(os.Args[1], ".")
}
