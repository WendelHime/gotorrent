package models

type Block struct {
	Index int
	Begin int
	Data  []byte
}

type Piece struct {
	Index    int
	Filepath string
	Hash     Hash
	Blocks   []Block
}
