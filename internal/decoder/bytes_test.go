package decoder

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadBytes(t *testing.T) {
	var tests = []struct {
		name   string
		assert func(t *testing.T, actual []byte, err error)
		setup  func() (io.Reader, int)
	}{
		{
			name: "read 1 byte",
			assert: func(t *testing.T, actual []byte, err error) {
				assert.Nil(t, err)
				assert.Equal(t, []byte{0x01}, actual)
			},
			setup: func() (io.Reader, int) {
				return bytes.NewBuffer([]byte{0x01}), 1
			},
		},
		{
			name: "reading more bytes than available should return EOF error",
			assert: func(t *testing.T, actual []byte, err error) {
				if assert.Error(t, err) {
					assert.Equal(t, io.EOF, err)
				}
			},
			setup: func() (io.Reader, int) {
				return bytes.NewBuffer([]byte{0x01}), 2
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r, n := tt.setup()
			actual, err := ReadBytes(r, n)
			tt.assert(t, actual, err)
		})
	}
}
