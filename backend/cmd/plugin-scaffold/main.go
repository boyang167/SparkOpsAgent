package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
)

func main() {
	id := flag.String("id", "", "plugin id (required)")
	name := flag.String("name", "", "display name (defaults to id)")
	out := flag.String("out", "", "output directory (omit to write file tree to stdout)")
	flag.Parse()

	if *id == "" {
		fmt.Fprintln(os.Stderr, "usage: plugin-scaffold --id <id> [--name <name>] [-out <dir>]")
		os.Exit(2)
	}

	files, err := install.GenerateScaffold(*id, *name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scaffold: %v\n", err)
		os.Exit(1)
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	if *out == "" {
		for _, name := range names {
			fmt.Printf("===== %s =====\n", name)
			fmt.Print(string(files[name]))
			if len(files[name]) == 0 || files[name][len(files[name])-1] != '\n' {
				fmt.Println()
			}
		}
		return
	}

	for _, name := range names {
		dest := filepath.Join(*out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "scaffold: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(dest, files[name], 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "scaffold: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "wrote %d files under %s\n", len(names), *out)
}
