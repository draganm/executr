// +build ignore

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Println("Long-running binary started")
	
	// Default duration is 60 seconds
	duration := 60 * time.Second
	if len(os.Args) > 1 {
		if d, err := time.ParseDuration(os.Args[1]); err == nil {
			duration = d
		}
	}
	
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	timer := time.NewTimer(duration)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	
	for {
		select {
		case <-timer.C:
			fmt.Printf("Completed after %v\n", time.Since(startTime))
			os.Exit(0)
		case <-ticker.C:
			fmt.Printf("Still running... (%v elapsed)\n", time.Since(startTime))
		case sig := <-sigChan:
			fmt.Printf("Received signal %v after %v\n", sig, time.Since(startTime))
			os.Exit(130) // Standard exit code for SIGINT
		}
	}
}