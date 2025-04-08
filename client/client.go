package client

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"keepGoing/core"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

func ClientMain(monitor *core.Monitor) {
	fmt.Println("Client started")
	go func() {
		for {
			x, y := robotgo.Location()
			totalWidth, totalHeight := core.CalcWidthHeight(monitor)
			workDisplayNum := core.GetWorkDisplay(monitor)
			keepGoing := core.DetectKeepGoing(x, y, monitor, totalWidth, totalHeight, workDisplayNum)
			if keepGoing {
				monitor.PeerConn.Write([]byte("keepGoing"))
			}
			time.Sleep(1000 * time.Millisecond)
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
	robotgo.Move(0, 0)
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
		procHookedEvent(monitor, event, &prevMousePos)
	}
}

func procHookedEvent(monitor *core.Monitor, event *hook.Event, prevMousePos *core.Vec2) {
	switch event.Kind {
	case hook.MouseMove:
		// x, y := robotgo.Location()
		fmt.Printf("get ev : %d, %d\n", event.X, event.Y)
		// fmt.Printf("cur %d, %d\n", x, y)
		// x = x + int(event.X)
		// y = y + int(event.Y)
		// x := int(event.X)
		// y := int(event.Y)
		// fmt.Printf("Mouse moved to: %d, %d\n", x, y)
		robotgo.Move(prevMousePos.X+int(event.X), prevMousePos.Y+int(event.Y))
		prevMousePos.X = prevMousePos.X + int(event.X)
		prevMousePos.Y = prevMousePos.Y + int(event.Y)
		if prevMousePos.X < monitor.Displays[0].Min.X {
			prevMousePos.X = monitor.Displays[0].Min.X
		}
		if prevMousePos.Y < monitor.Displays[0].Min.Y {
			prevMousePos.Y = monitor.Displays[0].Min.Y
		}
		if prevMousePos.X > monitor.Displays[0].W {
			prevMousePos.X = monitor.Displays[0].W
		}
		if prevMousePos.Y > monitor.Displays[0].H {
			prevMousePos.Y = monitor.Displays[0].H
		}
		fmt.Printf("prevMousePos.X: %d, prevMousePos.Y: %d\n", prevMousePos.X, prevMousePos.Y)
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
