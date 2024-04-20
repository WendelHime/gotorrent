package models

type Block struct {
	Index int
	Begin int
	Data  []byte
}

type Piece struct {
	Index  int
	Hash   Hash
	Blocks []Block
}
