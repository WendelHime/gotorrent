package models

type Metafile struct {
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list"`
	Info         Info       `bencode:"info"`
	InfoHash     Hash       `bencode:"-"`
}

type Info struct {
	Name         string `bencode:"name"`
	Length       int    `bencode:"length"`
	PieceLength  int    `bencode:"piece length"`
	Pieces       string `bencode:"pieces"`
	PiecesHashes []Hash `bencode:"-"`
	Files        []File `bencode:"files,omitempty"`
}

type File struct {
	Length int    `bencode:"length"`
	Path   string `bencode:"path"`
}

type Hash struct {
	Hash []byte
}

func (h Hash) String() string {
	return string(h.Hash)
}
