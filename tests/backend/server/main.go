// This fixture server binds loopback only and serves disposable probe payloads.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
)

func main() {
	root := flag.String("root", "", "fixture directory")
	ready := flag.String("ready-file", "", "write loopback URL here")
	flag.Parse()
	if *root == "" || *ready == "" {
		fmt.Fprintln(os.Stderr, "root and ready-file are required")
		os.Exit(2)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer listener.Close()
	data, _ := json.Marshal(map[string]string{"url": "http://" + listener.Addr().String()})
	if err := os.WriteFile(*ready, data, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := http.Serve(listener, http.FileServer(http.Dir(*root))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
