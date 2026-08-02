package main

import (
	"os"

	"github.com/amolofeev/yt/internal/commands"
)

func main() {
	os.Exit(commands.Run())
}
