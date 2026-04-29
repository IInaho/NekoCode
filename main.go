package main

import (
	"fmt"
	"os"
	"strings"

	"primusbot/bot"
	"primusbot/tui"
)

func main() {
	if len(os.Args) > 1 {
		runNonInteractive()
		return
	}

	tui.Run()
}

func runNonInteractive() {
	b := bot.New()
	input := strings.Join(os.Args[1:], " ")

	fmt.Println("> " + input)

	output, err := b.RunAgent(input, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(output)
}
