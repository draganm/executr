package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	app := &cli.App{
		Name:  "executr",
		Usage: "A tool for managing and executing commands",
		Action: func(c *cli.Context) error {
			fmt.Println("Running command")
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error("Error running app", "error", err)
		os.Exit(1)
	}

}
