// +build ignore

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello from success binary")
	fmt.Fprintln(os.Stderr, "This is stderr output")
	
	// Print arguments if any
	if len(os.Args) > 1 {
		fmt.Printf("Arguments: %v\n", os.Args[1:])
	}
	
	// Print environment variables if TEST_ENV is set
	if val := os.Getenv("TEST_ENV"); val != "" {
		fmt.Printf("TEST_ENV=%s\n", val)
	}
	
	os.Exit(0)
}