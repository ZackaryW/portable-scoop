// Command prepare-tools builds a verified private helper bundle using Go readers.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"portscoop/internal/toolbundle"
)

func main() {
	pin := flag.String("pins", "backend/tools/pins.json", "tool provenance and digests")
	cache := flag.String("cache", ".work/tool-cache", "verified artifact cache")
	dest := flag.String("destination", "", "new bundle destination")
	flag.Parse()
	if *dest == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "a new -destination is required")
		os.Exit(2)
	}
	p, err := toolbundle.ReadPins(*pin)
	if err == nil {
		_, err = toolbundle.Prepare(context.Background(), p, *cache, *dest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(*dest)
}
