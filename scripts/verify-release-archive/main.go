// Command verify-release-archive checks the final packaged artifact without
// installing it or executing downloaded release binaries.
package main

import (
	"flag"
	"fmt"
	"os"

	"portscoop/internal/selfupdate"
)

func main() {
	archive := flag.String("archive", "", "completed release ZIP")
	flag.Parse()
	if *archive == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: verify-release-archive -archive <release.zip>")
		os.Exit(2)
	}
	if err := selfupdate.VerifyArchive(*archive); err != nil {
		fmt.Fprintln(os.Stderr, "release archive verification:", err)
		os.Exit(1)
	}
	fmt.Println("Portscoop release archive verified")
}
