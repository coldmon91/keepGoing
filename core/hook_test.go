package core

import (
	"fmt"
	"testing"

	hook "github.com/robotn/gohook"
)

func TestHook(t *testing.T) {
	hook.Register(hook.KeyDown, []string{"ctrl", "c"}, func(e hook.Event) {
		fmt.Printf("KeyDown: %v\n", e)
		hook.End()
	})
	hook.Register(hook.KeyDown, []string{"c"}, func(e hook.Event) {
		fmt.Printf("KeyDown: %v\n", e)

		e.Rawcode = 0
	})
	s := hook.Start()
	<-hook.Process(s)
}
