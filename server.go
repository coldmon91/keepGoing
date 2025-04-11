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

			// 클라이언트로부터 받은 디스플레이 정보 파싱
			var peerDisplayInfo *core.DisplayInfo

			if monitor.PeerConn != nil {
				// 클라이언트 디스플레이 정보 파싱
				displayInfoBuffer := make([]byte, core.BufferSize)
				r, err := monitor.PeerConn.Read(displayInfoBuffer)
				if err != nil || r == 0 {
					fmt.Println("클라이언트 디스플레이 정보를 읽을 수 없습니다. 기본값을 사용합니다:", err)
					// 기본값 사용
					peerDisplayInfo = &core.DisplayInfo{
						Id: 0,
						Min: core.Vec2{
							X: 0,
							Y: 0,
						},
						W: 1920,
						H: 1080,
					}
				} else {
					// 수신된 JSON 파싱
					peerDisplayInfo = &core.DisplayInfo{}
					err = json.Unmarshal(displayInfoBuffer[:r], peerDisplayInfo)
					if err != nil {
						fmt.Println("클라이언트 디스플레이 정보 파싱 오류:", err)
						// 오류 시 기본값 사용
						peerDisplayInfo = &core.DisplayInfo{
							Id: 0,
							Min: core.Vec2{
								X: 0,
								Y: 0,
							},
							W: 1920,
							H: 1080,
						}
					} else {
						fmt.Printf("클라이언트 디스플레이 정보: %+v\n", peerDisplayInfo)
					}
				}
			} else {
				// 연결이 없는 경우 (테스트 모드)
				peerDisplayInfo = &core.DisplayInfo{
					Id: 0,
					Min: core.Vec2{
						X: 0,
						Y: 0,
					},
					W: 1920,
					H: 1080,
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
					if string(readMsg) == "keepGoing" {
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
