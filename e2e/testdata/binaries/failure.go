// +build ignore

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Starting failure binary")
	fmt.Fprintln(os.Stderr, "ERROR: Intentional failure")
	
	// Exit with specific code based on first argument
	exitCode := 1
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &exitCode)
	}
	
	fmt.Printf("Exiting with code %d\n", exitCode)
	os.Exit(exitCode)
}