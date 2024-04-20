package models

type PeerMessage struct {
	ID      MessageID
	Length  int
	Payload []byte
}

type Peer struct {
	PiecesWanted int
	Addr         Addr
	PeerID       string
	HavePieces   map[int]struct{}
}
