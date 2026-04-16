package main

import (
	"fmt"
	"os"

	"clickup-tui/cmd"
	"clickup-tui/pkg/logger"
)

func main() {
	if err := logger.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to setup logger: %v\n", err)
	}
	defer logger.Close()

	cmd.Execute()
}
