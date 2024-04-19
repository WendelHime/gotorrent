package decoder

import (
	"io"
	"strings"
	"testing"

	"github.com/WendelHime/gotorrent/internal/shared/models"
	"github.com/stretchr/testify/assert"
)

func TestMetainfoDecoder(t *testing.T) {
	decoder := NewDecoder()

	// write test cases
	var tests = []struct {
		name          string
		assert        func(t *testing.T, actual models.Metafile, err error)
		givenMetafile func() io.Reader
	}{
		{
			name: "valid metainfo",
			assert: func(t *testing.T, actual models.Metafile, err error) {
				assert.Nil(t, err)
				assert.Equal(t, "http://tracker.example.com", actual.Announce)
				assert.Equal(t, [][]string{{"http://tracker.example.com", "http://backup-tracker.com"}}, actual.AnnounceList)
				assert.Equal(t, "Torrent_Folder", actual.Info.Name)
				assert.Equal(t, 32768, actual.Info.PieceLength)
				assert.Equal(t, 90000, actual.Info.Length)
				assert.Equal(t, "0123456789abcdef01230000000000000000000000000000000000000000", actual.Info.Pieces)
				assert.Equal(t, []models.File{{Path: "subfolder1/file1.txt", Length: 1000}, {Path: "subfolder2/file2.txt", Length: 2000}}, actual.Info.Files)
				assert.Equal(t, "&\x02\xc7\xf4u7\xb6\xdcmWKv3\xc4w0QSe\xed", actual.InfoHash.String())
				assert.Equal(t, "0123456789abcdef0123", actual.Info.PiecesHashes[0].String())
				assert.Equal(t, "00000000000000000000", actual.Info.PiecesHashes[1].String())
				assert.Equal(t, "00000000000000000000", actual.Info.PiecesHashes[2].String())
			},
			givenMetafile: func() io.Reader {
				var b strings.Builder
				b.WriteString("d")
				b.WriteString("8:announce26:http://tracker.example.com")
				b.WriteString("13:announce-list")
				b.WriteString("ll26:http://tracker.example.com25:http://backup-tracker.comee")
				b.WriteString("10:created by15:MyTorrentClient")
				b.WriteString("4:info")
				b.WriteString("d")
				b.WriteString("6:lengthi90000e")
				b.WriteString("4:name")
				b.WriteString("14:Torrent_Folder")
				b.WriteString("12:piece lengthi32768e")
				b.WriteString("6:pieces60:0123456789abcdef01230000000000000000000000000000000000000000")
				b.WriteString("5:files")
				b.WriteString("l")
				b.WriteString("d4:path20:subfolder1/file1.txt6:lengthi1000ee")
				b.WriteString("d4:path20:subfolder2/file2.txt6:lengthi2000ee")
				b.WriteString("e")
				b.WriteString("e")
				b.WriteString("e")
				return strings.NewReader(b.String())
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			actual, err := decoder.Decode(tt.givenMetafile())
			tt.assert(t, actual, err)
		})
	}
}
