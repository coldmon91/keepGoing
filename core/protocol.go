package core

type MsgType int

const (
	MsgTypeSignal MsgType = iota
	MsgTypeEvent  MsgType = iota
	MsgTypeInform MsgType = iota
)

type Message struct {
	MsgType MsgType
	Data    []byte
}
type MouseEventMsg struct {
	MsgType   MsgType
	EventType uint8
	X         int16
	Y         int16
}

type KeyEventMsg struct {
	MsgType   MsgType
	EventType uint8
	RawCode   uint16
	KeyChar   rune
}
