package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	args := os.Args
	if len(args) < 2 || len(args) > 2 {
		fmt.Println("invalid input")
		os.Exit(1)
	}

	filename := args[1]

	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Unable to read a file:", filename)
	}

	io.Copy(os.Stdout, file)
}
