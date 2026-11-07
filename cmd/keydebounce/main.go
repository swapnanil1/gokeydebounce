// Command keydebounce filters rapid duplicate keyboard presses on Linux.
package main

import (
	"fmt"
	"os"

	"github.com/swapnanil1/gokeydebounce/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "keydebounce:", err)
		os.Exit(1)
	}
}
