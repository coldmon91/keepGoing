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

// 마우스 움직임 보정 상수
const (
	// 마우스 움직임 증폭 계수 (델타값에 곱해짐)
	MOUSE_MOVEMENT_AMPLIFIER_X = 1.0
	MOUSE_MOVEMENT_AMPLIFIER_Y = 1.0
)

func ClientMain(monitor *core.Monitor) {
	fmt.Println("Client started")
	go func() {
		var currentMousePos, previousMousePos core.Vec2
		for {
			currentMousePos.X, currentMousePos.Y = robotgo.Location()
			totalWidth, totalHeight := core.CalcWidthHeight(monitor)
			workDisplayNum := core.GetWorkDisplay(monitor)
			keepGoing := core.DetectKeepGoing(currentMousePos, previousMousePos, monitor.Settings, &monitor.Displays[workDisplayNum], totalWidth, totalHeight)
			if keepGoing {
				monitor.PeerConn.Write([]byte("keepGoing"))
			}
			previousMousePos = currentMousePos
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
		toString := string(readBuffer[:r])
		if toString == "keepGoing" {
			fmt.Println("recved keepGoing")

			// 서버에 준비 상태를 알림
			monitor.PeerConn.Write([]byte("monitor_ready"))

			// 최신 Monitor 객체 전송 (디스플레이 정보 포함)
			monitor.Displays = core.GetScreenSizes() // 최신 화면 정보로 업데이트

			b, err := json.Marshal(monitor)
			if err != nil {
				fmt.Println("Error marshalling monitor info:", err)
				continue
			}
			_, err = monitor.PeerConn.Write(b)
			if err != nil {
				fmt.Println("Error sending monitor info:", err)
				continue
			}
			fmt.Printf("[client] Monitor 정보 %d bytes 전송\n", len(b))
			continue
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

		// 서버와 클라이언트 화면 크기 비율에 따른 추가 스케일링 계수 계산
		// 기본값은 1.0(동일한 크기일 경우)
		scaleX := 1.0
		scaleY := 1.0

		// 화면 크기에 비례하여 마우스 이동 스케일링
		if len(monitor.Displays) > 0 && monitor.PeerDisplayInfo != nil {
			if monitor.PeerDisplayInfo.W > 0 && monitor.Displays[0].W > 0 {
				scaleX = float64(monitor.PeerDisplayInfo.W) / float64(monitor.Displays[0].W)
			}
			if monitor.PeerDisplayInfo.H > 0 && monitor.Displays[0].H > 0 {
				scaleY = float64(monitor.PeerDisplayInfo.H) / float64(monitor.Displays[0].H)
			}
		}

		// 마우스 움직임 증폭 적용 (기본 증폭 계수 × 화면 크기 비율)
		adjustedDeltaX := int(float64(event.X) * MOUSE_MOVEMENT_AMPLIFIER_X * scaleX)
		adjustedDeltaY := int(float64(event.Y) * MOUSE_MOVEMENT_AMPLIFIER_Y * scaleY)

		if core.DEBUG {
			fmt.Printf("원본 델타값: X=%d, Y=%d\n", event.X, event.Y)
			fmt.Printf("화면 비율: X=%f, Y=%f\n", scaleX, scaleY)
			fmt.Printf("증폭된 델타값: X=%d, Y=%d\n", adjustedDeltaX, adjustedDeltaY)
		}

		robotgo.Move(prevMousePos.X+adjustedDeltaX, prevMousePos.Y+adjustedDeltaY)
		prevMousePos.X = prevMousePos.X + adjustedDeltaX
		prevMousePos.Y = prevMousePos.Y + adjustedDeltaY

		// 화면 경계 처리
		if len(monitor.Displays) > 0 {
			display := monitor.Displays[0]
			if prevMousePos.X < display.Min.X {
				prevMousePos.X = display.Min.X
			}
			if prevMousePos.Y < display.Min.Y {
				prevMousePos.Y = display.Min.Y
			}
			if prevMousePos.X > display.Min.X+display.W {
				prevMousePos.X = display.Min.X + display.W
			}
			if prevMousePos.Y > display.Min.Y+display.H {
				prevMousePos.Y = display.Min.Y + display.H
			}
		}
		fmt.Printf("prevMousePos.X: %d, prevMousePos.Y: %d\n", prevMousePos.X, prevMousePos.Y)
	case hook.MouseDown:
		fmt.Printf("Mouse button %d pressed at: %d, %d\n", event.Button, event.X, event.Y)
		if event.Button == 1 {
			robotgo.MouseDown("left")
		}
		if event.Button == 2 {
			robotgo.MouseDown("right")
		}
		if event.Button == 3 {
			robotgo.MouseDown("middle")
		}
	case hook.MouseUp:
		fmt.Printf("Mouse button %d released at: %d, %d\n", event.Button, event.X, event.Y)
		if event.Button == 1 {
			robotgo.MouseUp("left")
		}
		if event.Button == 2 {
			robotgo.MouseUp("right")
		}
		if event.Button == 3 {
			robotgo.MouseUp("middle")
		}
	case hook.MouseWheel:
		fmt.Printf("Mouse wheel moved: %d, %d\n", event.X, event.Y)
		robotgo.Scroll(int(event.X), int(event.Y))
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
