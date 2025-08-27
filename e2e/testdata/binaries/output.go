// +build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	// Generate lots of output for truncation testing
	lines := 1000
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil {
			lines = n
		}
	}
	
	// Generate stdout
	for i := 1; i <= lines; i++ {
		// Make lines long enough to exceed 1MB quickly
		fmt.Printf("STDOUT Line %05d: %s\n", i, strings.Repeat("x", 100))
	}
	
	// Generate stderr
	for i := 1; i <= lines/2; i++ {
		fmt.Fprintf(os.Stderr, "STDERR Line %05d: %s\n", i, strings.Repeat("e", 100))
	}
	
	fmt.Println("=== END OF OUTPUT ===")
	os.Exit(0)
}