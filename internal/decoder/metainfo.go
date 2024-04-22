package decoder

import (
	"crypto/sha1"
	"io"
	"log/slog"
	"strings"

	"github.com/WendelHime/gotorrent/internal/shared/models"
	"github.com/zeebo/bencode"
)

type MetafileDecoder interface {
	Decode(io.Reader) (models.Metafile, error)
}

type decoder struct{}

func NewDecoder() MetafileDecoder {
	return decoder{}
}

// serialization struct the represents the structure of a .torrent file
// it is not immediately usable, so it can be converted to a TorrentFile struct
type bencodeTorrent struct {
	// URL of tracker server to get peers from
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list"`
	// Info is parsed as a RawMessage to ensure that the final info_hash is
	// correct even in the case of the info dictionary being an unexpected shape
	Info bencode.RawMessage `bencode:"info"`
}

func (decoder) Decode(torrent io.Reader) (models.Metafile, error) {
	var response models.Metafile
	var bt bencodeTorrent
	err := bencode.NewDecoder(torrent).Decode(&bt)
	if err != nil {
		slog.Error("failed to decode torrent: %v", err)
		return response, err
	}

	infoHash := calculateInfoHash(bt.Info)
	response.Announce = bt.Announce
	response.AnnounceList = bt.AnnounceList
	response.InfoHash = models.Hash{Hash: infoHash}
	err = bencode.NewDecoder(strings.NewReader(string(bt.Info))).Decode(&response.Info)
	if err != nil {
		slog.Error("failed to decode torrent info: %v", err)
		return response, err
	}

	response.Info.PiecesHashes, err = calculatePiecesHashes(response.Info.Pieces)
	if err != nil {
		slog.Error("failed to calculate pieces hashes: %v", err)
		return response, err
	}

	if response.Info.Length > 0 {
		response.Info.Files = []models.File{{Length: response.Info.Length, Path: []string{response.Info.Name}}}
	}

	return response, nil
}

func calculateInfoHash(info []byte) [20]byte {
	return sha1.Sum(info)
}

func calculatePiecesHashes(pieces string) ([]models.Hash, error) {
	piecesHashes := make([]models.Hash, 0)

	reader := strings.NewReader(pieces)
	for {
		hash := make([]byte, 20)
		_, err := reader.Read(hash)
		if err != nil && err != io.EOF {
			return []models.Hash{}, err
		}
		if err == io.EOF {
			break
		}
		piecesHashes = append(piecesHashes, models.Hash{Hash: [20]byte(hash)})
	}

	return piecesHashes, nil
}
