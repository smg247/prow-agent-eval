package main

import (
	"os"

	"github.com/smg247/prow-agent-eval/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
