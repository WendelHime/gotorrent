package models

import "net"

type PeerMessage struct {
	ID      MessageID
	Payload []byte
	Length  int
}

type Peer struct {
	Addr         Addr
	Port         int
	Conn         net.Conn
	PeerID       string
	HavePieces   map[int]struct{}
	PiecesWanted int
}
