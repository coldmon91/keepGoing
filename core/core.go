package core

import (
	"fmt"
	"net"
	"time"

	"github.com/go-vgo/robotgo"
)

var DEBUG = true

const BufferSize = 1024
const MouseEventCollectionDuration = 60 * time.Millisecond // 마우스 이벤트 수집 시간 (60ms)

type ScreenDirection int

const (
	Left   ScreenDirection = 0
	Right  ScreenDirection = 1
	Top    ScreenDirection = 2
	Bottom ScreenDirection = 3
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
		// MouseObj          MouseObject
		PeerConn        net.Conn      `json:"-"` // JSON 직렬화에서 제외
		Displays        []DisplayInfo `json:"displays"`
		PeerDisplayInfo *DisplayInfo  `json:"server_display_info,omitempty"` // 서버 디스플레이 정보 추가
	}

	Settings struct {
		Mode          string          `json:"mode"`
		PeerScreenDir ScreenDirection `json:"peer_screen_dir"`
	}
)

func (s *Settings) String() string {
	return fmt.Sprintf("Mode: %s, ScreenLoc: %d", s.Mode, s.PeerScreenDir)
}

func DetectKeepGoing(mousePos Vec2, prevMousePos Vec2, settings *Settings, screenSize *DisplayInfo, totalWidth, totalHeight int) bool {
	if settings.PeerScreenDir == Right {
		if mousePos.X == (totalWidth - 1) {
			if prevMousePos.X < mousePos.X {
				fmt.Printf("마우스가 오른쪽 끝에 도달했습니다: %d, %d\n", mousePos.X, mousePos.Y)
				return true
			}
		}
	} else if settings.PeerScreenDir == Left {
		if mousePos.X == 0 {
			if prevMousePos.X > mousePos.X {
				fmt.Printf("마우스가 왼쪽 끝에 도달했습니다: %d, %d\n", mousePos.X, mousePos.Y)
				return true
			}
		}
	} else if settings.PeerScreenDir == Top {
		if mousePos.Y == 0 {
			if prevMousePos.Y > mousePos.Y {
				fmt.Printf("마우스가 위쪽 끝에 도달했습니다: %d, %d\n", mousePos.X, mousePos.Y)
				return true
			}
		}
	} else if settings.PeerScreenDir == Bottom {
		if mousePos.Y == (screenSize.H - 1) {
			if prevMousePos.Y < mousePos.Y {
				fmt.Printf("마우스가 아래쪽 끝에 도달했습니다: %d, %d\n", mousePos.X, mousePos.Y)
				return true
			}
		}
	}
	return false
}

func CalcWidthHeight(monitor *Monitor) (int, int) {
	totalWidth := 0
	totalHeight := 0
	if monitor.Settings.PeerScreenDir == Left || monitor.Settings.PeerScreenDir == Right {
		for _, display := range monitor.Displays {
			totalWidth += display.W
		}
	} else if monitor.Settings.PeerScreenDir == Top || monitor.Settings.PeerScreenDir == Bottom {
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
