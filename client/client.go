package client

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"keepGoing/core"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

func ClientMain(monitor *core.Monitor) {
	go func() {
		for {
			x, y := robotgo.Location()
			totalWidth, totalHeight := core.CalcWidthHeight(monitor)
			workDisplayNum := core.GetWorkDisplay(monitor)
			keepGoing := core.DetectKeepGoing(x, y, monitor, totalWidth, totalHeight, workDisplayNum)
			if keepGoing {
				monitor.PeerConn.Write([]byte("keepGoing"))
			}
		}
	}()
	eventsPolling(monitor)
}

func eventsPolling(monitor *core.Monitor) {
	fmt.Printf("Polling events on %s\n", monitor.Settings.Mode)
	readBuffer := make([]byte, core.BufferSize)
	event := &hook.Event{}
	x, y := robotgo.Location()
	prevMousePos := core.Vec2{X: int(x), Y: int(y)}
	for {
		r, err := monitor.PeerConn.Read(readBuffer)
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			break
		}
		if r == 0 {
			fmt.Println("Connection closed by server")
			break
		}
		fmt.Printf("[client] Read %d bytes\n", r)
		decoder := gob.NewDecoder(bytes.NewBuffer(readBuffer))
		var msg core.Message
		err = decoder.Decode(&msg)
		if err != nil {
			fmt.Println("Error decoding message:", err)
			continue
		}
		err = json.Unmarshal(msg.Data, event)
		if err != nil {
			fmt.Println("Error unmarshalling event:", err)
			continue
		}
		procHookedEvent(event, &prevMousePos)
	}
}

func procHookedEvent(event *hook.Event, prevMousePos *core.Vec2) {
	switch event.Kind {
	case hook.MouseMove:
		fmt.Printf("Mouse moved to: %d, %d\n", event.X, event.Y)
		x := 0
		if event.X > 0 {
			x = prevMousePos.X + int(event.X)
		} else {
			x = prevMousePos.X - int(event.X)
		}
		y := 0
		if event.Y > 0 {
			y = prevMousePos.Y + int(event.Y)
		} else {
			y = prevMousePos.Y - int(event.Y)
		}
		robotgo.Move(int(x), int(y))
		prevMousePos.X = int(x)
		prevMousePos.Y = int(y)
	case hook.MouseDown:
		fmt.Printf("Mouse button %d pressed at: %d, %d\n", event.Button, event.X, event.Y)
		robotgo.MouseDown(event.Button)
	case hook.MouseUp:
		fmt.Printf("Mouse button %d released at: %d, %d\n", event.Button, event.X, event.Y)
		robotgo.MouseUp(event.Button)
	case hook.KeyDown:
		fmt.Printf("Key %d pressed\n", event.Rawcode)
		robotgo.KeyDown(hook.RawcodetoKeychar(event.Rawcode))
	case hook.KeyUp:
		fmt.Printf("Key %d released\n", event.Rawcode)
		robotgo.KeyUp(hook.RawcodetoKeychar(event.Rawcode))
	default:
		fmt.Printf("Unknown event type: %d\n", event.Kind)
	}
}
