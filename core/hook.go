package core

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

var skipMouseMove = false

func StartHooking(monitor *Monitor, peerDisplayInfo *DisplayInfo, hookChannel chan []byte) {
	if monitor.Settings.PeerScreenDir == Right {
		skipMouseMove = true
		robotgo.Move(peerDisplayInfo.Min.X, peerDisplayInfo.Min.Y)
	}
	x, y := robotgo.Location()
	prevMousePos := Vec2{X: int(x), Y: int(y)}

	// 현재 작업 중인 디스플레이 확인
	workDisplayNum := GetWorkDisplay(monitor)
	if workDisplayNum == -1 {
		workDisplayNum = 0 // 기본값 설정
	}

	// 현재 모니터와 클라이언트(Peer) 모니터 크기 비율 계산
	currentDisplay := monitor.Displays[workDisplayNum]
	widthRatio := float64(peerDisplayInfo.W) / float64(currentDisplay.W)
	heightRatio := float64(peerDisplayInfo.H) / float64(currentDisplay.H)

	if DEBUG {
		fmt.Printf("모니터 비율 계산: 가로 %.2f, 세로 %.2f\n", widthRatio, heightRatio)
	}

	// 마우스 이벤트 수집 관련 변수
	var accumulatedDeltaX int16 = 0
	var accumulatedDeltaY int16 = 0
	var timer *time.Timer
	var timerMutex sync.Mutex
	var isTimerRunning bool = false

	// 마우스 이벤트 전송 함수
	sendMouseEvent := func() {
		timerMutex.Lock()
		defer timerMutex.Unlock()

		isTimerRunning = false

		// 델타가 없으면 전송하지 않음
		if accumulatedDeltaX == 0 && accumulatedDeltaY == 0 {
			return
		}

		// 수집된 델타 값으로 이벤트 생성
		e := hook.Event{
			Kind: hook.MouseMove,
			X:    accumulatedDeltaX,
			Y:    accumulatedDeltaY,
		}

		if DEBUG {
			fmt.Printf("최종 누적 델타 전송 - deltaX: %d, deltaY: %d\n", accumulatedDeltaX, accumulatedDeltaY)
		}

		// 이벤트 전송
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		message := Message{
			MsgType: hook.MouseMove,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(message)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}
		hookChannel <- bytesBuffer.Bytes()

		// 누적 델타 초기화
		accumulatedDeltaX = 0
		accumulatedDeltaY = 0
	}

	hook.Register(hook.MouseMove, []string{}, func(e hook.Event) {
		if skipMouseMove {
			skipMouseMove = false
			return
		}
		centerX := peerDisplayInfo.Min.X + peerDisplayInfo.W/2
		centerY := peerDisplayInfo.Min.Y + peerDisplayInfo.H/2

		if prevMousePos.X != int(e.X) || prevMousePos.Y != int(e.Y) {
			// 델타값 계산
			deltaX := e.X - int16(prevMousePos.X)
			deltaY := e.Y - int16(prevMousePos.Y)
			prevMousePos.X = int(e.X)
			prevMousePos.Y = int(e.Y)

			// 클라이언트 화면 크기에 맞게 델타값 조정
			adjustedDeltaX := int16(float64(deltaX) * widthRatio)
			adjustedDeltaY := int16(float64(deltaY) * heightRatio)

			// 델타값 누적
			timerMutex.Lock()
			accumulatedDeltaX += adjustedDeltaX
			accumulatedDeltaY += adjustedDeltaY

			// 타이머가 실행 중이 아니면 새로 시작
			if !isTimerRunning {
				isTimerRunning = true
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(MouseEventCollectionDuration, sendMouseEvent)
			}
			timerMutex.Unlock()

			if DEBUG {
				fmt.Printf("원본 deltaX: %d, deltaY: %d\n", deltaX, deltaY)
				fmt.Printf("조정된 deltaX: %d, deltaY: %d\n", adjustedDeltaX, adjustedDeltaY)
				fmt.Printf("누적된 deltaX: %d, deltaY: %d\n", accumulatedDeltaX, accumulatedDeltaY)
			} else {
				fmt.Printf("deltaX: %d, deltaY: %d\n", adjustedDeltaX, adjustedDeltaY)
			}
		} else {
			// fmt.Printf("마우스 위치가 변경되지 않았습니다: %d, %d\n", e.X, e.Y)
			return
		}

		skipMouseMove = true
		prevMousePos.X = centerX
		prevMousePos.Y = centerY
		robotgo.Move(centerX, centerY)
	})
	hook.Register(hook.MouseDown, []string{}, func(e hook.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		mouseMsg := Message{
			MsgType: hook.MouseDown,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(mouseMsg)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}
		hookChannel <- bytesBuffer.Bytes()
	})
	hook.Register(hook.MouseUp, []string{}, func(e hook.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		mouseMsg := Message{
			MsgType: hook.MouseUp,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(mouseMsg)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}
		hookChannel <- bytesBuffer.Bytes()
	})

	hook.Register(hook.KeyDown, []string{}, func(e hook.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		keyMsg := Message{
			MsgType: hook.KeyDown,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(keyMsg)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}

		hookChannel <- bytesBuffer.Bytes()
	})
	hook.Register(hook.KeyUp, []string{}, func(e hook.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		keyMsg := Message{
			MsgType: hook.KeyUp,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(keyMsg)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}
		hookChannel <- bytesBuffer.Bytes()
	})
	hook.Register(hook.MouseWheel, []string{}, func(e hook.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		mouseMsg := Message{
			MsgType: hook.MouseWheel,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(mouseMsg)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}
		hookChannel <- bytesBuffer.Bytes()
	})
	hook.Register(hook.MouseDrag, []string{}, func(e hook.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Println("JSON 인코딩 오류:", err)
			return
		}
		mouseMsg := Message{
			MsgType: hook.MouseDrag,
			Data:    data,
		}
		bytesBuffer := new(bytes.Buffer)
		err = gob.NewEncoder(bytesBuffer).Encode(mouseMsg)
		if err != nil {
			fmt.Println("메시지 인코딩 오류:", err)
			return
		}
		hookChannel <- bytesBuffer.Bytes()
	})
	s := hook.Start()
	<-hook.Process(s)
}
