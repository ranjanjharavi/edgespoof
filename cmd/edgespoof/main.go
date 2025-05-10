package main

import "os"

func main() {
	if len(os.Args) < 2 {
		printUsage()
	}
	if err := run(os.Args[1:]); err != nil {
		printError(err)
		os.Exit(1)
	}
}
