package tracker

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/WendelHime/gotorrent/internal/shared/models"
	"github.com/jackpal/bencode-go"
	"github.com/stretchr/testify/assert"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func TestGetPeers(t *testing.T) {
	var tests = []struct {
		name   string
		setup  func(t *testing.T) (Tracker, models.Metafile)
		assert func(t *testing.T, actual []models.Peer, err error)
	}{
		{
			name: "get peers with success",
			setup: func(t *testing.T) (Tracker, models.Metafile) {
				tracker := NewTracker("http://tracker.example.com", "01234567891012345678").WithHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
					assert.Equal(t, "http://tracker.example.com?compact=1&downloaded=0&event=started&info_hash=01234567891012345678&left=100&peer_id=01234567891012345678&port=6881&uploaded=0", req.URL.String())
					ipAddr := net.ParseIP("192.168.100.100")
					assert.NotNil(t, ipAddr)
					ipBytes := ipAddr.To4()
					assert.NotNil(t, ipBytes)

					portBytes := make([]byte, 2)
					binary.BigEndian.PutUint16(portBytes, uint16(6889))
					peerBytes := append(ipBytes, portBytes...)
					response := peersResponse{
						Interval: 60,
						Peers:    string(peerBytes),
					}
					resp := bytes.NewBuffer([]byte{})
					err := bencode.Marshal(resp, response)
					assert.Nil(t, err)

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(resp),
					}
				}))
				hash := models.Hash{
					Hash: []byte("01234567891012345678"),
				}
				return tracker, models.Metafile{
					InfoHash: hash,
					Info: models.Info{
						Length: 100,
						PiecesHashes: []models.Hash{
							{Hash: []byte("01234567891012345678")},
						},
					},
				}
			},
			assert: func(t *testing.T, actual []models.Peer, err error) {
				assert.Nil(t, err)
				assert.Len(t, actual, 1)
				assert.Equal(t, net.IPv4(192, 168, 100, 100), actual[0].Addr.IP)
				assert.Equal(t, 6889, int(actual[0].Addr.Port))
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tracker, metafile := tt.setup(t)
			actual, err := tracker.GetPeers(metafile)
			tt.assert(t, actual, err)
		})
	}
}
