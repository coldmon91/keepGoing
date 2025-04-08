package main

import (
	"encoding/json"
	"fmt"
	"keepGoing/client"
	core "keepGoing/core"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

type ExecArg struct {
	Mode     string `arg:"-m,--mode" default:"server" help:"server or client"`
	PeerAddr string `arg:"-s,--server" help:"server address"`
	Debug    bool   `arg:"-d,--debug" help:"debug mode"`
}

func main() {
	args := ExecArg{}
	arg.MustParse(&args)
	displays := core.GetScreenSizes()
	fmt.Printf("args : %v\n", args)
	core.DEBUG = args.Debug

	settings := &core.Settings{}
	settings.Mode = args.Mode
	if settings.Mode == "client" {
		if args.PeerAddr == "" {
			fmt.Println("서버 주소를 입력하세요. --server <ip>:<port>")
			return
		}
	}
	settings.PeerScreenLoc = core.Right // TODO: get from user

	myMonitor := core.Monitor{
		Settings: settings,
		Displays: displays,
		MouseObj: core.MouseObject{},
	}
	if args.PeerAddr == "" {
		args.PeerAddr = "0.0.0.0:50310"
	}
	// port := 50310
	stopChan := StartCapture(settings.Mode, &myMonitor, args.PeerAddr)

	time.Sleep(60 * time.Second)
	core.StopCapture(stopChan)
}

func StartCapture(mode string, myMonitor *core.Monitor, peerAddress string) (stopChan chan bool) {
	addrs := strings.Split(peerAddress, ":")
	var port int
	if len(addrs) == 1 {
	} else if len(addrs) == 2 {
		port, _ = strconv.Atoi(addrs[1])
	} else {
		fmt.Println("서버 주소 형식이 잘못되었습니다. <ip>:<port> 형식으로 입력하세요.")
		return nil
	}
	stopChan = make(chan bool)
	var conn net.Conn
	if myMonitor.Settings.Mode == "server" {
		if !core.DEBUG {
			conn = tcpListen(port)
			if conn == nil {
				fmt.Println("서버 연결 오류")
				return nil
			}
			myMonitor.PeerConn = conn
			b, e := json.Marshal(myMonitor.Settings)
			if e != nil {
				fmt.Println("Settings JSON 변환 오류:", e)
				return nil
			}
			myMonitor.PeerConn.Write(b)
			fmt.Printf("[server] %d bytes sent %s\n", len(b), string(b))
		}
		fmt.Println("키보드와 마우스 캡처를 시작합니다...")
		fmt.Printf("서버 설정: %s\n", myMonitor.Settings.String())
		go core.CaptureMouse(myMonitor, stopChan)
	} else if myMonitor.Settings.Mode == "client" {
		conn = tcpConnect(peerAddress)
		if conn == nil {
			fmt.Println("서버 연결 오류")
			return nil
		}
		fmt.Println("서버에 연결되었습니다:", peerAddress)
		myMonitor.PeerConn = conn

		b := make([]byte, core.BufferSize)
		r, err := myMonitor.PeerConn.Read(b)
		if err != nil {
			fmt.Println("서버에서 설정을 읽는 중 오류 발생:", err)
			return nil
		}
		if r == 0 {
			fmt.Println("서버 연결이 종료되었습니다.")
			return nil
		}
		fmt.Printf("[client] Read %d bytes\n", r)
		peerSettings := core.Settings{}
		err = json.Unmarshal(b[:r], &peerSettings)
		if err != nil {
			fmt.Println("Settings JSON 변환 오류:", err)
			return nil
		}
		fmt.Println("클라이언트가 서버에서 설정을 받았습니다:", string(b))
		if peerSettings.PeerScreenLoc == core.Left {
			myMonitor.Settings.PeerScreenLoc = core.Right
		} else if peerSettings.PeerScreenLoc == core.Right {
			myMonitor.Settings.PeerScreenLoc = core.Left
		} else if peerSettings.PeerScreenLoc == core.Top {
			myMonitor.Settings.PeerScreenLoc = core.Bottom
		} else if peerSettings.PeerScreenLoc == core.Bottom {
			myMonitor.Settings.PeerScreenLoc = core.Top
		}
		fmt.Printf("클라이언트 설정: %s\n", myMonitor.Settings.String())
		client.ClientMain(myMonitor)
	}

	return stopChan
}

func tcpListen(port int) net.Conn {
	address := fmt.Sprintf("0.0.0.0:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Printf("서버를 시작할 수 없습니다: %v\n", err)
		return nil
	}
	defer listener.Close()
	fmt.Printf("서버가 %s에서 시작되었습니다.\n", address)
	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("클라이언트 연결을 수락할 수 없습니다: %v\n", err)
		return nil
	}
	fmt.Printf("클라이언트가 연결되었습니다: %s\n", conn.RemoteAddr().String())
	return conn
}

func tcpConnect(address string) net.Conn {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("서버에 연결할 수 없습니다: %v\n", err)
		return nil
	}
	fmt.Printf("서버에 연결되었습니다: %s\n", address)
	return conn
}
