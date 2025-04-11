package main

import (
	"encoding/json"
	"fmt"
	"keepGoing/core"
	"time"

	"github.com/go-vgo/robotgo"
)

const PollingRate = 50 * time.Millisecond

func ServerMain(monitor *core.Monitor, stopChan <-chan bool) {
	totalWidth, totalHeight := core.CalcWidthHeight(monitor)

	// 클라이언트로부터 받은 Monitor 객체 저장용 변수
	var peerMonitor *core.Monitor

	for {
		workDisplayNum := core.GetWorkDisplay(monitor)
		x, y := robotgo.Location()
		fmt.Printf("마우스 위치: %d, %d (display %d)\n", x, y, workDisplayNum)
		keepGoing := core.DetectKeepGoing(x, y, monitor, totalWidth, totalHeight, workDisplayNum)
		if keepGoing && !core.DEBUG {
			fmt.Printf("keepGoing\n")
			// start hooking
			readMsg := make([]byte, core.BufferSize)
			hookChannel := make(chan []byte)

			fmt.Printf("keepGoing msg to client\n")
			monitor.PeerConn.Write([]byte("keepGoing"))

			fmt.Println("waitting for message from client")

			// 이미 Monitor 객체를 교환했기 때문에 여기서는 응답만 확인
			r, err := monitor.PeerConn.Read(readMsg)
			if err != nil {
				fmt.Println("클라이언트 응답 수신 오류:", err)
				return
			}
			if r == 0 {
				fmt.Println("클라이언트 연결이 종료되었습니다.")
				return
			}

			// 클라이언트로부터 이미 받은 Monitor 객체에서 디스플레이 정보 가져오기
			var peerDisplayInfo *core.DisplayInfo

			if string(readMsg[:r]) == "monitor_ready" {
				// 클라이언트가 준비되었다는 메시지를 보내면
				// 클라이언트 Monitor 객체 수신
				peerMonitorBuffer := make([]byte, core.BufferSize*4)
				r, err := monitor.PeerConn.Read(peerMonitorBuffer)
				if err != nil {
					fmt.Println("클라이언트 Monitor 정보 수신 오류:", err)
					return
				}

				peerMonitor = &core.Monitor{}
				err = json.Unmarshal(peerMonitorBuffer[:r], peerMonitor)
				if err != nil {
					fmt.Println("클라이언트 Monitor JSON 변환 오류:", err)
					// 오류 시 기본값 사용
					peerDisplayInfo = &core.DisplayInfo{
						Id:  0,
						Min: core.Vec2{X: 0, Y: 0},
						W:   1920,
						H:   1080,
					}
				} else {
					if peerMonitor.Displays != nil && len(peerMonitor.Displays) > 0 {
						fmt.Printf("클라이언트 디스플레이 정보: %+v\n", peerMonitor.Displays[0])
						peerDisplayInfo = &peerMonitor.Displays[0]
					} else {
						// 디스플레이 정보가 없는 경우 기본값 사용
						peerDisplayInfo = &core.DisplayInfo{
							Id:  0,
							Min: core.Vec2{X: 0, Y: 0},
							W:   1920,
							H:   1080,
						}
					}
				}
			} else {
				// 예전 프로토콜과의 호환성 유지
				fmt.Printf("[server] 클라이언트로부터 %d bytes 수신: %s\n", r, string(readMsg[:r]))
				// 임시 기본값 설정
				peerDisplayInfo = &core.DisplayInfo{
					Id:  0,
					Min: core.Vec2{X: 0, Y: 0},
					W:   1920,
					H:   1080,
				}
			}

			go core.StartHooking(monitor, peerDisplayInfo, hookChannel)
			keepGoingChan := make(chan bool)
			go func() { // waitting for message from client
				for {
					// receive message from client
					r, err := monitor.PeerConn.Read(readMsg)
					if err != nil {
						fmt.Println("연결 읽기 오류:", err)
						return
					}
					if r == 0 {
						fmt.Println("연결이 종료되었습니다.")
						return
					}
					fmt.Printf("[server] %d bytes 읽음\n", r)
					if string(readMsg[:r]) == "keepGoing" {
						fmt.Println("keepGoing from client")
						keepGoingChan <- true
						return
					}
				}
			}()
			for keepGoing {
				select {
				case msg := <-hookChannel:
					_, err := monitor.PeerConn.Write(msg)
					if err != nil {
						fmt.Println("메시지 전송 오류:", err)
						return
					}
				case <-keepGoingChan:
					keepGoing = false
					fmt.Println("keepGoing from client")
				case <-stopChan:
					fmt.Println("stopChan 수신")
					return
				}
			}
		}
		monitor.MouseObj.PreviousMousePos.X = x
		monitor.MouseObj.PreviousMousePos.Y = y
		time.Sleep(PollingRate)
	}
}
