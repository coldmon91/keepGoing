package core

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

var DEBUG = true

const BufferSize = 1024

var PollingRate = 500 * time.Millisecond

type PeerScreenLocation int

const (
	Left   PeerScreenLocation = 0
	Right  PeerScreenLocation = 1
	Top    PeerScreenLocation = 2
	Bottom PeerScreenLocation = 3
)

type (
	Vec2 struct {
		X int
		Y int
	}
	MouseObject struct {
		PreviousMousePos Vec2
	}

	Monitor struct {
		Settings *Settings
		MouseObj MouseObject
		PeerConn net.Conn
		Displays []DisplayInfo `json:"displays"`
	}

	Settings struct {
		Mode          string             `json:"mode"`
		PeerScreenLoc PeerScreenLocation `json:"peer_screen_loc"`
	}
)

func (s *Settings) String() string {
	return fmt.Sprintf("Mode: %s, ScreenLoc: %d", s.Mode, s.PeerScreenLoc)
}

func DetectKeepGoing(x, y int /*mouse pos*/, monitor *Monitor, totalWidth, totalHeight, workDisplayNum int) bool {

	settings := monitor.Settings
	mouseObj := monitor.MouseObj
	screenSize := monitor.Displays[workDisplayNum]

	if settings.PeerScreenLoc == Right {
		if x == (totalWidth - 1) {
			if mouseObj.PreviousMousePos.X < x {
				fmt.Printf("마우스가 오른쪽 끝에 도달했습니다: %d, %d\n", x, y)
				return true
			}
		}
	} else if settings.PeerScreenLoc == Left {
		if x == 0 {
			if mouseObj.PreviousMousePos.X > x {
				fmt.Printf("마우스가 왼쪽 끝에 도달했습니다: %d, %d\n", x, y)
				return true
			}
		}
	} else if settings.PeerScreenLoc == Top {
		if y == 0 {
			if mouseObj.PreviousMousePos.Y > y {
				fmt.Printf("마우스가 위쪽 끝에 도달했습니다: %d, %d\n", x, y)
				return true
			}
		}
	} else if settings.PeerScreenLoc == Bottom {
		if y == (screenSize.H - 1) {
			if mouseObj.PreviousMousePos.Y < y {
				fmt.Printf("마우스가 아래쪽 끝에 도달했습니다: %d, %d\n", x, y)
				return true
			}
		}
	}
	return false
}

func startHooking(hookChannel chan []byte) {
	x, y := robotgo.Location()
	prevMousePos := Vec2{X: int(x), Y: int(y)}
	hook.Register(hook.MouseMove, []string{}, func(e hook.Event) {
		if prevMousePos.X != int(e.X) || prevMousePos.Y != int(e.Y) {
			deltaX := e.X - int16(prevMousePos.X)
			deltaY := e.Y - int16(prevMousePos.Y)
			prevMousePos.X = int(e.X)
			prevMousePos.Y = int(e.Y)
			e.X = deltaX
			e.Y = deltaY
			fmt.Printf("deltaX: %d, deltaY: %d\n", deltaX, deltaY)
		}
		// e.Kind == MouseMove == 9
		// fmt.Printf("마우스 이동: %d, %d\n", e.X, e.Y)
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
	s := hook.Start()
	<-hook.Process(s)
}

func CalcWidthHeight(monitor *Monitor) (int, int) {
	totalWidth := 0
	totalHeight := 0
	if monitor.Settings.PeerScreenLoc == Left || monitor.Settings.PeerScreenLoc == Right {
		for _, display := range monitor.Displays {
			totalWidth += display.W
		}
	} else if monitor.Settings.PeerScreenLoc == Top || monitor.Settings.PeerScreenLoc == Bottom {
		for _, display := range monitor.Displays {
			totalHeight += display.H
		}
	}
	return totalWidth, totalHeight
}
func GetWorkDisplay(monitor *Monitor) int {
	x, y := robotgo.Location()
	for _, display := range monitor.Displays {
		if x >= display.Min.X && x <= display.Min.X+display.W &&
			y >= display.Min.Y && y <= display.Min.Y+display.H {
			fmt.Printf("디스플레이 #%d 범위 내에 있습니다: %d, %d\n", display.Id, x, y)
			return display.Id
		}
	}
	return -1
}

func CaptureMouse(monitor *Monitor, stopChan <-chan bool) {
	totalWidth, totalHeight := CalcWidthHeight(monitor)
	for {
		workDisplayNum := GetWorkDisplay(monitor)
		x, y := robotgo.Location()
		fmt.Printf("마우스 위치: %d, %d (display %d)\n", x, y, workDisplayNum)
		keepGoing := DetectKeepGoing(x, y, monitor, totalWidth, totalHeight, workDisplayNum)
		if keepGoing && DEBUG {
			hookChannel := make(chan []byte)
			go startHooking(hookChannel)
			for keepGoing {
				select {
				case <-hookChannel:
				}
			}
		}
		if keepGoing && !DEBUG {
			fmt.Printf("keepGoing\n")
			// start hooking
			readMsg := make([]byte, BufferSize)
			hookChannel := make(chan []byte)
			go startHooking(hookChannel)
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

func StopCapture(stopChan chan<- bool) {
	stopChan <- true
	fmt.Println("키보드와 마우스 캡처를 중지합니다.")
}
