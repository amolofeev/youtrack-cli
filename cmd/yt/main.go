package main

import (
	"os"

	"github.com/amolofeev/prompt-and-pray/internal/commands"
)

func main() {
	os.Exit(commands.Run())
}
