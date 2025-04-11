package core

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

// 프로그램에 의한 마우스 이동인지 확인하는 플래그 추가
var isMovingProgrammatically = false

var DEBUG = true

const BufferSize = 1024

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
		Settings          *Settings
		MouseObj          MouseObject
		PeerConn          net.Conn      `json:"-"` // JSON 직렬화에서 제외
		Displays          []DisplayInfo `json:"displays"`
		ServerDisplayInfo *DisplayInfo  `json:"server_display_info,omitempty"` // 서버 디스플레이 정보 추가
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

var skipMouseMove = false

func StartHooking(monitor *Monitor, peerDisplayInfo *DisplayInfo, hookChannel chan []byte) {
	if monitor.Settings.PeerScreenLoc == Right {
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

	hook.Register(hook.MouseMove, []string{}, func(e hook.Event) {
		if skipMouseMove {
			skipMouseMove = false
			return
		}
		if prevMousePos.X != int(e.X) || prevMousePos.Y != int(e.Y) {
			// 델타값 계산
			deltaX := e.X - int16(prevMousePos.X)
			deltaY := e.Y - int16(prevMousePos.Y)
			prevMousePos.X = int(e.X)
			prevMousePos.Y = int(e.Y)

			// 클라이언트 화면 크기에 맞게 델타값 조정
			adjustedDeltaX := int16(float64(deltaX) * widthRatio)
			adjustedDeltaY := int16(float64(deltaY) * heightRatio)

			// 조정된 델타값 적용
			e.X = adjustedDeltaX
			e.Y = adjustedDeltaY

			if DEBUG {
				fmt.Printf("원본 deltaX: %d, deltaY: %d\n", deltaX, deltaY)
				fmt.Printf("조정된 deltaX: %d, deltaY: %d\n", adjustedDeltaX, adjustedDeltaY)
			} else {
				fmt.Printf("deltaX: %d, deltaY: %d\n", adjustedDeltaX, adjustedDeltaY)
			}
		} else {
			// fmt.Printf("마우스 위치가 변경되지 않았습니다: %d, %d\n", e.X, e.Y)
			return
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

func StopCapture(stopChan chan<- bool) {
	stopChan <- true
	fmt.Println("키보드와 마우스 캡처를 중지합니다.")
}
