package tracker

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/WendelHime/gotorrent/internal/shared/models"
	"github.com/jackpal/bencode-go"
)

type HTTPGetter struct {
	client   *http.Client
	peerID   string
	interval int
}

func NewHTTPGetter(client *http.Client, peerID string) PeersGetter {
	return &HTTPGetter{client: client, peerID: peerID}
}

func (h *HTTPGetter) GetPeers(announce string, metafile models.Metafile) ([]models.Peer, error) {
	tracker, err := url.Parse(announce)
	if err != nil {
		return nil, err
	}

	query := tracker.Query()
	query.Add("info_hash", metafile.InfoHash.String())
	query.Add("peer_id", h.peerID)
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(metafile.Info.Length))
	query.Add("compact", "1")
	query.Add("event", "started")
	tracker.RawQuery = query.Encode()

	response, err := h.client.Get(tracker.String())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %s", response.Status)

	}

	peersResp, err := decodeHTTPResponse(response.Body)
	if err != nil {
		return nil, err
	}

	h.interval = peersResp.Interval
	peers := make([]models.Peer, len(peersResp.Peers))
	for i, p := range peersResp.Peers {
		peers[i] = models.Peer{
			Addr:         p.Addr,
			HavePieces:   make(map[int]struct{}),
			PiecesWanted: len(metafile.Info.PiecesHashes),
		}
	}

	return peers, nil
}

func decodeHTTPResponse(response io.Reader) (peersWithAddresses, error) {
	resp := peersResponse{}
	err := bencode.Unmarshal(response, &resp)
	if err != nil {
		return peersWithAddresses{}, err
	}

	peers := make([]models.Peer, 0)
	for i := 0; i < len(resp.Peers); i += 6 {
		addr := models.Addr{
			IP:   net.ParseIP(fmt.Sprintf("%d.%d.%d.%d", resp.Peers[i], resp.Peers[i+1], resp.Peers[i+2], resp.Peers[i+3])),
			Port: uint16(int(resp.Peers[i+4])<<8 | int(resp.Peers[i+5])),
		}
		peers = append(peers, models.Peer{Addr: addr})
	}

	return peersWithAddresses{Peers: peers, Interval: resp.Interval}, nil
}
