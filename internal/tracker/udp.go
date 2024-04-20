package tracker

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/WendelHime/gotorrent/internal/decoder"
	"github.com/WendelHime/gotorrent/internal/shared/models"
)

type UDPGetter struct {
	interval int
	peerID   string
}

func NewUDPGetter(peerID string) PeersGetter {
	return UDPGetter{peerID: peerID}
}

type Event int

const (
	EventNone Event = iota
	EventCompleted
	EventStarted
	EventStopped
)

func (u UDPGetter) GetPeers(announce string, metafile models.Metafile) ([]models.Peer, error) {
	tracker, err := url.Parse(announce)
	if err != nil {
		return nil, err
	}

	peerPort, err := strconv.Atoi(tracker.Port())
	if err != nil {
		return nil, err
	}

	ip, err := net.LookupIP(tracker.Hostname())
	if err != nil {
		return nil, err
	}

	raddr := net.UDPAddr{
		IP:   ip[0],
		Port: peerPort,
	}

	conn, err := net.DialUDP("udp", nil, &raddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	err = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return nil, err
	}

	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, err
	}

	connectRequest := bytes.NewBuffer(nil)
	binary.Write(connectRequest, binary.BigEndian, uint64(0x41727101980)) // protocol_id
	binary.Write(connectRequest, binary.BigEndian, uint32(0))             // action
	binary.Write(connectRequest, binary.BigEndian, transactionID)         // transaction_id

	_, err = conn.Write(connectRequest.Bytes())
	if err != nil {
		return nil, err
	}

	resp, err := decoder.ReadBytes(conn, 16)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if len(resp) < 16 {
		return nil, errors.New("invalid response")
	}
	action := binary.BigEndian.Uint32(resp[:4])
	txID := binary.BigEndian.Uint32(resp[4:8])
	connectionID := binary.BigEndian.Uint64(resp[8:16])
	if txID != transactionID {
		return nil, errors.New("invalid transaction ID")
	}

	if action != 0 {
		return nil, errors.New("invalid action")
	}

	transactionID, err = generateTransactionID()
	if err != nil {
		return nil, err
	}

	peersRequested := -1
	req := bytes.NewBuffer(nil)
	binary.Write(req, binary.BigEndian, uint64(connectionID)) // connection_id
	binary.Write(req, binary.BigEndian, uint32(1))            // action
	binary.Write(req, binary.BigEndian, transactionID)        // transaction_id
	req.Write(metafile.InfoHash.Hash)
	req.Write([]byte(u.peerID))
	binary.Write(req, binary.BigEndian, uint64(0))                    // downloaded
	binary.Write(req, binary.BigEndian, uint64(metafile.Info.Length)) // left
	binary.Write(req, binary.BigEndian, uint64(0))                    // uploaded
	binary.Write(req, binary.BigEndian, uint32(EventNone))            // event
	binary.Write(req, binary.BigEndian, uint32(0))                    // ip
	binary.Write(req, binary.BigEndian, uint32(0))                    // key
	binary.Write(req, binary.BigEndian, int32(peersRequested))        // num_want
	binary.Write(req, binary.BigEndian, uint16(peerPort))             // port

	// sending announce request
	_, err = conn.Write(req.Bytes())
	if err != nil {
		return nil, err
	}

	resp, err = decoder.ReadBytes(conn, 2048)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if len(resp) < 20 {
		return nil, errors.New("invalid response")
	}

	action = binary.BigEndian.Uint32(resp[:4])
	txID = binary.BigEndian.Uint32(resp[4:8])
	// interval := binary.BigEndian.Uint32(resp[8:12])
	// leechers := binary.BigEndian.Uint32(buf[12:16])
	// seeders := binary.BigEndian.Uint32(buf[16:20])

	if txID != transactionID {
		return nil, errors.New("invalid transaction ID")
	}

	if action != 1 {
		return nil, errors.New("invalid action")
	}

	peerData := resp[20:]

	peers := make([]models.Peer, 0)

	for {
		addr := models.Addr{}
		err = addr.ReadFromBytes(peerData[:6])
		if err != nil {
			return nil, err
		}

		peer := models.Peer{
			Addr:         addr,
			HavePieces:   make(map[int]struct{}),
			PiecesWanted: len(metafile.Info.PiecesHashes),
		}

		peers = append(peers, peer)

		if len(peerData) < 6 {
			break
		}

		peerData = peerData[6:]
		if len(peerData) == 0 {
			break
		}
	}

	return peers, nil
}

func copyToSlice(dst []byte, src []byte, offset int) {
	for i, b := range src {
		dst[offset+i] = b
	}
}
func generateTransactionID() (uint32, error) {
	var transactionID uint32
	randomBytes := make([]byte, 4) // 4 bytes for a uint32

	// Read 4 random bytes from the crypto/rand package
	_, err := rand.Read(randomBytes)
	if err != nil {
		return 0, err
	}

	// Convert the random bytes to a uint32
	transactionID = binary.BigEndian.Uint32(randomBytes)

	return transactionID, nil
}
