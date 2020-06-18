package main

import (
	"github.com/gemalto/helm-spray/cmd"
	"os"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
