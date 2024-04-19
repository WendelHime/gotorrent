package decoder

import (
	"bytes"
	"crypto/sha1"
	"io"
	"log/slog"
	"strings"

	"github.com/WendelHime/gotorrent/internal/shared/models"
	"github.com/jackpal/bencode-go"
)

type MetafileDecoder interface {
	Decode(io.Reader) (models.Metafile, error)
}

type decoder struct{}

func NewDecoder() MetafileDecoder {
	return decoder{}
}

func (decoder) Decode(torrent io.Reader) (models.Metafile, error) {
	var response models.Metafile
	err := bencode.Unmarshal(torrent, &response)
	if err != nil {
		slog.Error("failed to decode torrent: %v", err)
		return response, err
	}

	infoHash, err := calculateInfoHash(response.Info)
	if err != nil {
		slog.Error("failed to calculate info hash: %v", err)
		return response, err
	}
	response.InfoHash = models.Hash{Hash: infoHash}

	response.Info.PiecesHashes, err = calculatePiecesHashes(response.Info.Pieces)
	if err != nil {
		slog.Error("failed to calculate pieces hashes: %v", err)
		return response, err
	}

	return response, nil
}

func calculateInfoHash(info models.Info) ([]byte, error) {
	var b bytes.Buffer
	err := bencode.Marshal(&b, info)
	if err != nil {
		return []byte{}, err
	}

	dst := sha1.Sum(b.Bytes())
	return dst[:], nil
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
		piecesHashes = append(piecesHashes, models.Hash{Hash: hash})
	}

	return piecesHashes, nil
}
