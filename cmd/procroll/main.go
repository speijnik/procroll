package main

import (
	"github.com/speijnik/procroll/internal/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
