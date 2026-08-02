package main

import (
	"os"

	"github.com/amolofeev/youtrack-cli/internal/commands"
)

func main() {
	os.Exit(commands.Run())
}
