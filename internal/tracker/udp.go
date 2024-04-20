package tracker

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
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

type Event uint32

const (
	EventNone Event = iota
	EventCompleted
	EventStarted
	EventStopped
)

type Action uint32

const (
	ActionConnect Action = iota
	ActionAnnounce
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
	fmt.Println("Connected to tracker", raddr.String())
	for n := 0; n < 8; n++ {
		fmt.Println("attempt", n)
		waitTime := int64(15 * math.Pow(2, float64(n)))
		err = conn.SetDeadline(time.Now().Add(time.Duration(waitTime) * time.Second))
		if err != nil {
			return nil, err
		}

		transactionID, err := generateTransactionID()
		if err != nil {
			return nil, err
		}

		connectRequest := ConnectRequest{
			ProtocolID:    0x41727101980,
			Action:        ActionConnect,
			TransactionID: transactionID,
		}

		fmt.Println("Sending connect request", connectRequest, raddr.String())
		_, err = conn.Write(connectRequest.Bytes())
		if err != nil {
			return nil, err
		}

		fmt.Println("Reading connect response", raddr.String())
		connectResponse, err := readConnectResponse(conn)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			}
			return nil, err
		}

		if connectResponse.TransactionID != connectRequest.TransactionID {
			return nil, errors.New("invalid transaction ID")
		}

		if connectResponse.Action != 0 {
			return nil, errors.New("invalid action")
		}

		transactionID, err = generateTransactionID()
		if err != nil {
			return nil, err
		}

		peersRequested := 100
		announceRequest := AnnounceRequest{
			ConnectionID:  connectResponse.ConnectionID,
			Action:        ActionAnnounce,
			TransactionID: transactionID,
			InfoHash:      metafile.InfoHash.String(),
			PeerID:        u.peerID,
			Downloaded:    0,
			Left:          uint64(metafile.Info.Length),
			Uploaded:      0,
			Event:         EventStarted,
			IP:            0,
			Key:           0,
			NumWant:       int32(peersRequested),
			Port:          uint16(peerPort),
		}

		fmt.Println("Sending announce request", announceRequest, raddr.String())
		// sending announce request
		b, err := announceRequest.Bytes()
		if err != nil {
			return nil, err
		}

		_, err = conn.Write(b)
		if err != nil {
			return nil, err
		}

		fmt.Println("Reading announce response", raddr.String())
		announceResponse, err := readAnnounceResponse(conn, peersRequested)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			}
			return nil, err
		}

		if announceResponse.TransactionID != announceRequest.TransactionID {
			return nil, errors.New("invalid transaction ID")
		}

		if announceResponse.Action != ActionAnnounce {
			return nil, errors.New("invalid action")
		}
		return announceResponse.Peers, nil

	}

	return []models.Peer{}, nil
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

func readConnectResponse(conn net.Conn) (ConnectResponse, error) {
	resp, err := decoder.ReadBytes(conn, 16)
	if err != nil && err != io.EOF {
		return ConnectResponse{}, err
	}

	if len(resp) < 16 {
		return ConnectResponse{}, errors.New("invalid response")
	}
	action := binary.BigEndian.Uint32(resp[:4])
	txID := binary.BigEndian.Uint32(resp[4:8])
	connectionID := binary.BigEndian.Uint64(resp[8:16])

	return ConnectResponse{Action: Action(action), TransactionID: txID, ConnectionID: connectionID}, nil
}

type AnnounceRequest struct {
	ConnectionID  uint64
	Action        Action
	TransactionID uint32
	InfoHash      string
	PeerID        string
	Downloaded    uint64
	Left          uint64
	Uploaded      uint64
	Event         Event
	IP            uint32
	Key           uint32
	NumWant       int32
	Port          uint16
}

func (r *AnnounceRequest) Bytes() ([]byte, error) {
	req := bytes.NewBuffer(nil)
	binary.Write(req, binary.BigEndian, r.ConnectionID)   // connection_id
	binary.Write(req, binary.BigEndian, uint32(r.Action)) // action
	binary.Write(req, binary.BigEndian, r.TransactionID)  // transaction_id
	n, err := req.WriteString(r.InfoHash)                 // info_hash
	if err != nil {
		return nil, err
	}

	if n < 20 || n > 20 {
		return nil, errors.New("invalid info hash")
	}

	n, err = req.WriteString(r.PeerID) // peer_id
	if err != nil {
		return nil, err
	}

	if n < 20 || n > 20 {
		return nil, errors.New("invalid peer id")
	}
	binary.Write(req, binary.BigEndian, r.Downloaded)    // downloaded
	binary.Write(req, binary.BigEndian, r.Left)          // left
	binary.Write(req, binary.BigEndian, r.Uploaded)      // uploaded
	binary.Write(req, binary.BigEndian, uint32(r.Event)) // event
	binary.Write(req, binary.BigEndian, r.IP)            // ip
	binary.Write(req, binary.BigEndian, r.Key)           // key
	binary.Write(req, binary.BigEndian, r.NumWant)       // num_want
	binary.Write(req, binary.BigEndian, r.Port)          // port
	return req.Bytes(), nil
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

func readAnnounceResponse(conn net.Conn, peersWanted int) (AnnounceResponse, error) {
	resp, err := decoder.ReadBytes(conn, 20+6*peersWanted)
	if err != nil && err != io.EOF {
		return AnnounceResponse{}, err
	}

	if len(resp) < 20 {
		return AnnounceResponse{}, errors.New("invalid response")
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
		peers = append(peers, models.Peer{Addr: addr})
	}

	return AnnounceResponse{Action: Action(action), TransactionID: txID, Interval: interval, Leechers: leechers, Seeders: seeders, Peers: peers}, nil
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
