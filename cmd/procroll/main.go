package main

import (
	"os"

	"github.com/speijnik/procroll/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
