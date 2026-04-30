package main

import (
	"flag"
	"fmt"
	"os"

	"gd-wolf/internal/wl6"
)

func main() {
	dataDir := ""
	rest, err := parseArgs(os.Args[1:], map[string]*string{
		"-data":  &dataDir,
		"--data": &dataDir,
	})
	if err != nil {
		if err == flag.ErrHelp {
			usage()
			os.Exit(2)
		}
		fatal(err)
	}
	if len(rest) < 1 {
		usage()
		os.Exit(2)
	}

	files, err := openFiles(dataDir)
	if err != nil {
		fatal(err)
	}

	switch rest[0] {
	case "maps":
		if err := listMaps(files); err != nil {
			fatal(err)
		}
	case "map":
		if len(rest) != 2 {
			usage()
			os.Exit(2)
		}
		if err := dumpMap(files, rest[1]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: wolf-mapdump [-data path] maps")
	fmt.Fprintln(os.Stderr, "       wolf-mapdump [-data path] map <index>")
}

func parseArgs(args []string, stringFlags map[string]*string) ([]string, error) {
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help", "help":
			return nil, flag.ErrHelp
		}
		if dst, ok := stringFlags[arg]; ok {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}
			*dst = args[i+1]
			i++
			continue
		}
		rest = append(rest, arg)
	}
	return rest, nil
}

func openFiles(dataDir string) (*wl6.Files, error) {
	if dataDir == "" {
		files, _, err := wl6.OpenDefault()
		return files, err
	}
	return wl6.Open(dataDir)
}

func listMaps(files *wl6.Files) error {
	maps, err := files.Maps()
	if err != nil {
		return err
	}
	for _, m := range maps {
		fmt.Printf("%2d  %dx%d  %s\n", m.Index, m.Width, m.Height, m.Name)
	}
	return nil
}

func dumpMap(files *wl6.Files, arg string) error {
	var index int
	if _, err := fmt.Sscanf(arg, "%d", &index); err != nil {
		return fmt.Errorf("parse map index %q: %w", arg, err)
	}
	data, err := files.LoadMap(index)
	if err != nil {
		return err
	}
	fmt.Printf("map %d: %s (%dx%d)\n", index, data.Header.Name, data.Header.Width, data.Header.Height)
	fmt.Printf("plane0 cells: %d\n", len(data.Planes[0]))
	fmt.Printf("plane1 cells: %d\n", len(data.Planes[1]))
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
