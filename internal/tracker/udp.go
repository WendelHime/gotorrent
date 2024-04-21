package tracker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"strings"
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

type Event uint32

const (
	EventNone Event = iota
	EventCompleted
	EventStarted
	EventStopped
)

type Action uint32

const (
	ActionConnect  Action = 0
	ActionAnnounce Action = 1
)

func (u UDPGetter) GetPeers(announceURL string, metafile models.Metafile) ([]models.Peer, error) {
	tracker, err := url.Parse(announceURL)
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
	conn, connID, err := connect(raddr)
	if err != nil {
		return nil, err
	}

	announceResponse, err := announce(conn, connID, metafile, u.peerID)
	if err != nil {
		return nil, err
	}

	return announceResponse.Peers, nil
}

func connect(raddr net.UDPAddr) (net.Conn, uint64, error) {
	timeout := 15 * time.Second
	conn, err := net.DialTimeout("udp", raddr.String(), timeout)
	if err != nil {
		return nil, 0, err
	}
	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, 0, err
	}

	connectRequest := ConnectRequest{
		ProtocolID:    0x41727101980,
		Action:        ActionConnect,
		TransactionID: transactionID,
	}

	var n, retries int
	var connectResponse ConnectResponse

	for {
		retries++

		err = conn.SetWriteDeadline(time.Now().Add(timeout))
		if err != nil {
			return nil, 0, err
		}

		n, err = conn.Write(connectRequest.Bytes())
		if err != nil {
			return nil, 0, err
		}
		if n != len(connectRequest.Bytes()) {
			return nil, 0, errors.New("invalid write")
		}

		err = conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return nil, 0, err
		}
		connectResponse, n, err = readConnectResponse(conn)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				if retries > 8 {
					return nil, 0, errors.New("too many retries")
				}
				continue
			}
			return nil, 0, err
		}

		break
	}
	if n != 16 {
		return nil, 0, fmt.Errorf("invalid response: %d", n)
	}
	if connectResponse.TransactionID != connectRequest.TransactionID {
		return nil, 0, errors.New("invalid transaction ID")
	}

	if connectResponse.Action != 0 {
		return nil, 0, errors.New("invalid action")
	}

	return conn, connectResponse.ConnectionID, nil
}
func announce(conn net.Conn, connectionID uint64, metafile models.Metafile, peerID string) (AnnounceResponse, error) {
	timeout := 15 * time.Second
	transactionID, err := generateTransactionID()
	if err != nil {
		return AnnounceResponse{}, err
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	peersRequested := -1

	peerIDReader := strings.NewReader(peerID)
	p := make([]byte, 20)
	_, err = peerIDReader.Read(p)
	if err != nil {
		return AnnounceResponse{}, err
	}

	announceRequest := AnnounceRequest{
		ConnectionID:  connectionID,
		Action:        ActionAnnounce,
		TransactionID: transactionID,
		InfoHash:      metafile.InfoHash.Hash,
		PeerID:        [20]byte(p),
		Downloaded:    0,
		Left:          uint64(metafile.Info.Length),
		Uploaded:      0,
		Event:         EventNone,
		IP:            0,
		Key:           r.Uint32(),
		NumWant:       uint32(peersRequested),
		Port:          uint16(6881),
	}
	b, err := announceRequest.Bytes()
	if err != nil {
		return AnnounceResponse{}, err
	}

	var announceResponse AnnounceResponse
	var n, retries int

	for {
		retries++
		err = conn.SetWriteDeadline(time.Now().Add(timeout))
		if err != nil {
			return AnnounceResponse{}, err
		}
		// sending announce request
		n, err = conn.Write(b)
		if err != nil {
			return AnnounceResponse{}, err
		}

		if n != len(b) {
			return AnnounceResponse{}, errors.New("invalid write")
		}

		err = conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return AnnounceResponse{}, err
		}
		announceResponse, err = readAnnounceResponse(conn, len(metafile.Info.PiecesHashes))
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				if retries > 8 {
					return AnnounceResponse{}, errors.New("too many retries")
				}
				continue
			}
			return AnnounceResponse{}, err
		}
		break
	}

	if announceResponse.TransactionID != announceRequest.TransactionID {
		return AnnounceResponse{}, errors.New("invalid transaction ID")
	}

	if announceResponse.Action != ActionAnnounce {
		return AnnounceResponse{}, errors.New("invalid action")
	}
	return announceResponse, nil
}

type ConnectRequest struct {
	ProtocolID    uint64
	Action        Action
	TransactionID uint32
}

type ConnectResponse struct {
	Action        Action
	TransactionID uint32
	ConnectionID  uint64
}

func (request ConnectRequest) Bytes() []byte {
	connectRequest := bytes.NewBuffer(nil)
	binary.Write(connectRequest, binary.BigEndian, request.ProtocolID)     // protocol_id
	binary.Write(connectRequest, binary.BigEndian, uint32(request.Action)) // action
	binary.Write(connectRequest, binary.BigEndian, request.TransactionID)  // transaction_id
	return connectRequest.Bytes()
}

func readConnectResponse(conn net.Conn) (ConnectResponse, int, error) {
	resp, err := decoder.ReadBytes(conn, 16)
	if err != nil && err != io.EOF {
		return ConnectResponse{}, 0, err
	}

	n := len(resp)
	if n < 16 {
		return ConnectResponse{}, n, fmt.Errorf("invalid response: %d", n)
	}
	action := binary.BigEndian.Uint32(resp[:4])
	txID := binary.BigEndian.Uint32(resp[4:8])
	connectionID := binary.BigEndian.Uint64(resp[8:])

	return ConnectResponse{Action: Action(action), TransactionID: txID, ConnectionID: connectionID}, n, nil
}

type AnnounceRequest struct {
	ConnectionID  uint64
	Action        Action
	TransactionID uint32
	InfoHash      [20]byte
	PeerID        [20]byte
	Downloaded    uint64
	Left          uint64
	Uploaded      uint64
	Event         Event
	IP            uint32
	Key           uint32
	NumWant       uint32
	Port          uint16
}

func (r *AnnounceRequest) Bytes() ([]byte, error) {
	req := make([]byte, 98)
	binary.BigEndian.PutUint64(req[0:8], r.ConnectionID)    // connection_id
	binary.BigEndian.PutUint32(req[8:12], uint32(r.Action)) // action
	binary.BigEndian.PutUint32(req[12:16], r.TransactionID) // transaction_id
	copy(req[16:36], r.InfoHash[:])
	copy(req[36:56], r.PeerID[:])
	binary.BigEndian.PutUint64(req[56:64], r.Downloaded)    // downloaded
	binary.BigEndian.PutUint64(req[64:72], r.Left)          // left
	binary.BigEndian.PutUint64(req[72:80], r.Uploaded)      // uploaded
	binary.BigEndian.PutUint32(req[80:84], uint32(r.Event)) // event
	binary.BigEndian.PutUint32(req[84:88], r.IP)            // ip
	binary.BigEndian.PutUint32(req[88:92], r.Key)           // key
	binary.BigEndian.PutUint32(req[92:96], r.NumWant)       // num_want
	binary.BigEndian.PutUint16(req[96:98], r.Port)          // port
	return req, nil
}

type AnnounceResponse struct {
	Action        Action
	TransactionID uint32
	Interval      uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []models.Peer
}

func copyToSlice(dst []byte, src []byte, offset int) {
	for i, b := range src {
		dst[offset+i] = b
	}
}

func readAnnounceResponse(conn net.Conn, piecesWanted int) (AnnounceResponse, error) {
	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil && err != io.EOF {
		return AnnounceResponse{}, err
	}
	resp = resp[:n]

	if n < 20 {
		return AnnounceResponse{}, fmt.Errorf("invalid response: %d", n)
	}

	action := binary.BigEndian.Uint32(resp[:4])
	txID := binary.BigEndian.Uint32(resp[4:8])
	interval := binary.BigEndian.Uint32(resp[8:12])
	leechers := binary.BigEndian.Uint32(resp[12:16])
	seeders := binary.BigEndian.Uint32(resp[16:20])

	peers := make([]models.Peer, 0)
	peerData := resp[20:]
	for i := 0; i < len(peerData); i += 6 {
		addr := models.Addr{
			IP:   net.ParseIP(fmt.Sprintf("%d.%d.%d.%d", peerData[i], peerData[i+1], peerData[i+2], peerData[i+3])),
			Port: uint16(int(peerData[i+4])<<8 | int(peerData[i+5])),
		}
		p := models.Peer{
			Addr:         addr,
			HavePieces:   make(map[int]struct{}),
			PiecesWanted: piecesWanted,
		}
		peers = append(peers, p)
	}

	return AnnounceResponse{Action: Action(action), TransactionID: txID, Interval: interval, Leechers: leechers, Seeders: seeders, Peers: peers}, nil
}

func generateTransactionID() (uint32, error) {
	var transactionID uint32
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	transactionID = r.Uint32()
	return transactionID, nil
}
