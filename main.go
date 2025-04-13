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
	settings.PeerScreenDir = core.Right // TODO: get from user

	myMonitor := core.Monitor{
		Settings: settings,
		Displays: displays,
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

			// 서버의 Monitor 객체 전송
			b, e := json.Marshal(myMonitor)
			if e != nil {
				fmt.Println("Monitor JSON 변환 오류:", e)
				return nil
			}
			_, err := myMonitor.PeerConn.Write(b)
			if err != nil {
				fmt.Println("Monitor 정보 전송 오류:", err)
				return nil
			}
			fmt.Printf("[server] %d bytes sent, Monitor 객체를 클라이언트로 전송했습니다.\n", len(b))

			// 클라이언트의 Monitor 객체 수신
			peerMonitorBuffer := make([]byte, core.BufferSize*4) // Monitor 객체는 큰 데이터일 수 있어 버퍼 크기 증가
			r, err := myMonitor.PeerConn.Read(peerMonitorBuffer)
			if err != nil {
				fmt.Println("클라이언트 Monitor 정보 수신 오류:", err)
				return nil
			}
			if r == 0 {
				fmt.Println("클라이언트 연결이 종료되었습니다.")
				return nil
			}

			var peerMonitor core.Monitor
			err = json.Unmarshal(peerMonitorBuffer[:r], &peerMonitor)
			if err != nil {
				fmt.Println("클라이언트 Monitor JSON 변환 오류:", err)
				return nil
			}

			fmt.Printf("[server] 클라이언트 Monitor 정보 %d bytes 수신\n", r)

			// 클라이언트에서 받은 디스플레이 정보 저장
			if peerMonitor.Displays != nil && len(peerMonitor.Displays) > 0 {
				fmt.Printf("[server] 클라이언트 디스플레이 정보: %+v\n", peerMonitor.Displays[0])
				// ServerDisplayInfo 필드를 클라이언트의 기본 디스플레이 정보로 설정
				myMonitor.PeerDisplayInfo = &peerMonitor.Displays[0]
			}
		}
		fmt.Println("키보드와 마우스 캡처를 시작합니다...")
		fmt.Printf("서버 설정: %s\n", myMonitor.Settings.String())
		go ServerMain(myMonitor, stopChan)
	} else if myMonitor.Settings.Mode == "client" {
		conn = tcpConnect(peerAddress)
		if conn == nil {
			fmt.Println("서버 연결 오류")
			return nil
		}
		fmt.Println("서버에 연결되었습니다:", peerAddress)
		myMonitor.PeerConn = conn

		// 서버의 Monitor 객체 수신
		peerMonitorBuffer := make([]byte, core.BufferSize*4) // Monitor 객체는 큰 데이터일 수 있어 버퍼 크기 증가
		r, err := myMonitor.PeerConn.Read(peerMonitorBuffer)
		if err != nil {
			fmt.Println("서버에서 Monitor 정보를 읽는 중 오류 발생:", err)
			return nil
		}
		if r == 0 {
			fmt.Println("서버 연결이 종료되었습니다.")
			return nil
		}

		var peerMonitor core.Monitor
		err = json.Unmarshal(peerMonitorBuffer[:r], &peerMonitor)
		if err != nil {
			fmt.Println("서버 Monitor JSON 변환 오류:", err)
			return nil
		}
		fmt.Printf("[client] 서버 Monitor 정보 %d bytes 수신\n", r)

		// 서버의 설정 정보를 바탕으로 클라이언트의 화면 위치 설정
		if peerMonitor.Settings != nil {
			if peerMonitor.Settings.PeerScreenDir == core.Left {
				myMonitor.Settings.PeerScreenDir = core.Right
			} else if peerMonitor.Settings.PeerScreenDir == core.Right {
				myMonitor.Settings.PeerScreenDir = core.Left
			} else if peerMonitor.Settings.PeerScreenDir == core.Top {
				myMonitor.Settings.PeerScreenDir = core.Bottom
			} else if peerMonitor.Settings.PeerScreenDir == core.Bottom {
				myMonitor.Settings.PeerScreenDir = core.Top
			}
		}

		// 서버의 디스플레이 정보를 클라이언트의 ServerDisplayInfo 필드에 설정
		if peerMonitor.Displays != nil && len(peerMonitor.Displays) > 0 {
			myMonitor.PeerDisplayInfo = &peerMonitor.Displays[0]
			fmt.Printf("[client] 서버 디스플레이 정보 설정: %+v\n", *myMonitor.PeerDisplayInfo)
		}

		// 클라이언트의 Monitor 객체 전송
		b, e := json.Marshal(myMonitor)
		if e != nil {
			fmt.Println("Monitor JSON 변환 오류:", e)
			return nil
		}
		_, err = myMonitor.PeerConn.Write(b)
		if err != nil {
			fmt.Println("Monitor 정보 전송 오류:", err)
			return nil
		}
		fmt.Printf("[client] %d bytes sent, Monitor 객체를 서버로 전송했습니다.\n", len(b))

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
	return conn
}
