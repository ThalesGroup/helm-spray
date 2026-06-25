package main

import (
	"github.com/gemalto/helm-spray/v4/cmd"
	"os"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs(legacyPluginArgs(os.Args[1:]))
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func legacyPluginArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	if args[0] == "spray" {
		args = args[1:]
		if len(args) == 0 {
			return args
		}
	}

	valueFlags := map[string]bool{
		"--exclude":         true,
		"--prefix-releases": true,
		"--set":             true,
		"--set-file":        true,
		"--set-string":      true,
		"--target":          true,
		"--timeout":         true,
		"--values":          true,
		"--version":         true,
		"-f":                true,
		"-t":                true,
		"-x":                true,
	}
	subcommands := map[string]bool{
		"completion": true,
		"help":       true,
		"web":        true,
	}

	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	skipNext := false
	for _, arg := range args {
		if skipNext {
			flags = append(flags, arg)
			skipNext = false
			continue
		}
		if arg == "--" {
			return args
		}
		if subcommands[arg] {
			return args
		}
		if len(arg) > 0 && arg[0] == '-' {
			flags = append(flags, arg)
			if valueFlags[arg] {
				skipNext = true
			}
			continue
		}

		positionals = append(positionals, arg)
	}

	if len(positionals) == 0 {
		return flags
	}

	normalized := make([]string, 0, len(args)+1)
	normalized = append(normalized, flags...)
	normalized = append(normalized, "--")
	normalized = append(normalized, positionals...)
	return normalized
}
