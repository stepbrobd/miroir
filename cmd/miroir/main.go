package main

import "os"

var version = "dev"

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
