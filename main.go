package main

import (
	"os"

	"github.com/aryans1319/devdoctor/cmd"
)

func main() {
	// If first argument is "serve", start the web server
	// Otherwise run the CLI
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		startServer()
		return
	}

	cmd.Execute()
}