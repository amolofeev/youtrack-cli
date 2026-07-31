package main

import (
	"os"

	"github.com/amolofeev/prompt-and-pray/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
