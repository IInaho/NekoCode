package test

import (
	"fmt"
	"primusbot/bot"
	"testing"
)

func TestBot(t *testing.T) {
	bot := bot.New()

	bot.Chat("你好", func(token string, delta string) {
		fmt.Print(delta)
	}, nil)
}
