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

const BufferSize = 1024

var PollingRate = 50 * time.Millisecond

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
	}

	Settings struct {
		Mode          string             `json:"mode"`
		ScreenSize    Vec2               `json:"screen_size"`
		PeerScreenLoc PeerScreenLocation `json:"peer_screen_loc"`
	}
)

func (s *Settings) String() string {
	return fmt.Sprintf("Mode: %s, ScreenSize: (%d, %d), PeerScreenLoc: %d", s.Mode, s.ScreenSize.X, s.ScreenSize.Y, s.PeerScreenLoc)
}

func DetectKeepGoing(x, y int, monitor *Monitor) bool {
	settings := monitor.Settings
	mouseObj := monitor.MouseObj
	if settings.PeerScreenLoc == Right {
		if x == (settings.ScreenSize.X - 1) {
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
		if y == (settings.ScreenSize.Y - 1) {
			if mouseObj.PreviousMousePos.Y < y {
				fmt.Printf("마우스가 아래쪽 끝에 도달했습니다: %d, %d\n", x, y)
				return true
			}
		}
	}
	return false
}

func startHooking(hookChannel chan []byte) {
	hook.Register(hook.MouseMove, []string{}, func(e hook.Event) {
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

func CaptureMouse(monitor Monitor, stopChan <-chan bool) {
	for {
		x, y := robotgo.Location()
		keepGoing := DetectKeepGoing(x, y, &monitor)
		if keepGoing {
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
