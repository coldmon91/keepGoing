package core

import (
	"fmt"

	"github.com/kbinani/screenshot"
)

type DisplayInfo struct {
	Id  int
	Min Vec2
	W   int
	H   int
}

func GetScreenSizes() []DisplayInfo {
	n := screenshot.NumActiveDisplays()
	fmt.Printf("Found %d active displays\n", n)
	displayInfos := make([]DisplayInfo, n)
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		fmt.Printf("디스플레이 #%d: 위치 (%d, %d), 해상도 %dx%d\n",
			i, bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy())
		displayInfos[i] = DisplayInfo{
			Id: i,
			Min: Vec2{
				X: bounds.Min.X,
				Y: bounds.Min.Y,
			},
			W: bounds.Dx(),
			H: bounds.Dy(),
		}
	}
	return displayInfos
}
