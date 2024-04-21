package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/WendelHime/gotorrent/internal/shared/models"
)

type Tracker interface {
	GetPeers(models.Metafile) ([]models.Peer, error)
	WithHTTPClient(client *http.Client) Tracker
}

type PeersGetter interface {
	GetPeers(announce string, metafile models.Metafile) ([]models.Peer, error)
}

type tracker struct {
	AnnounceURL string
	PeerID      string
	HTTPClient  PeersGetter
	UDPClient   PeersGetter
}

func NewTracker(announceURL, peerID string) Tracker {
	return &tracker{
		AnnounceURL: announceURL,
		PeerID:      peerID,
		HTTPClient:  NewHTTPGetter(&http.Client{Timeout: 60 * time.Second}, peerID),
		UDPClient:   NewUDPGetter(peerID),
	}
}

func (t *tracker) WithHTTPClient(client *http.Client) Tracker {
	t.HTTPClient = NewHTTPGetter(client, t.PeerID)
	return t
}

type peersResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

type peersWithAddresses struct {
	Peers    []models.Peer
	Interval int
}

func (t *tracker) GetPeers(metafile models.Metafile) ([]models.Peer, error) {
	if t.AnnounceURL == "" {
		return nil, fmt.Errorf("announce url is empty")
	}
	switch {
	case strings.HasPrefix(t.AnnounceURL, "http"):
		return t.HTTPClient.GetPeers(t.AnnounceURL, metafile)
	case strings.HasPrefix(t.AnnounceURL, "udp"):
		return t.UDPClient.GetPeers(t.AnnounceURL, metafile)
	default:
		slog.Error("unsupported protocol", slog.String("announce-url", t.AnnounceURL))
		return nil, fmt.Errorf("unsupported protocol")
	}
}
