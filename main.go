package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"nekocode/bot"
	"nekocode/tui"
)

func main() {
	defer recoverPanic()

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

func recoverPanic() {
	if r := recover(); r != nil {
		stack := debug.Stack()
		logPath := fmt.Sprintf("nekocode-panic-%d.log", time.Now().Unix())
		msg := fmt.Sprintf("PANIC: %v\n\nStack:\n%s", r, string(stack))
		_ = os.WriteFile(logPath, []byte(msg), 0644)
		fmt.Fprintf(os.Stderr, "\nPANIC: %v\nStack saved to %s\n", r, logPath)
	}
}
