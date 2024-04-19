package tracker

import (
	"encoding/binary"
	"net"
	"net/url"
	"strconv"

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

	buf := make([]byte, 16)

	transactionID := 4609668
	binary.BigEndian.PutUint64(buf[0:], 0x41727101980)          // connection_id
	binary.BigEndian.PutUint32(buf[8:], 0)                      // action
	binary.BigEndian.PutUint32(buf[12:], uint32(transactionID)) // transaction_id

	_, err = conn.Write(buf)
	if err != nil {
		return nil, err
	}

	resp, err := decoder.ReadBytes(conn, 16)
	if err != nil {
		return nil, err
	}

	// action := binary.BigEndian.Uint32(resp[:4])
	// transaction_id := binary.BigEndian.Uint32(resp[4:8])
	connection_id := binary.BigEndian.Uint64(resp[8:])

	buf = make([]byte, 100)

	peersRequested := 100
	binary.BigEndian.PutUint64(buf[0:8], connection_id)                  // int64_t 	connection_id 	The connection id acquired from establishing the connection.
	binary.BigEndian.PutUint32(buf[8:12], 1)                             // int32_t 	action 	Action. in this case, 1 for announce. See actions.
	binary.BigEndian.PutUint32(buf[12:16], uint32(transactionID))        // int32_t 	transaction_id 	Randomized by client.
	copyToSlice(buf, metafile.InfoHash.Hash, 16)                         // int8_t[20] 	info_hash 	The info-hash of the torrent you want announce yourself in.
	copyToSlice(buf, []byte(u.peerID), 36)                               // int8_t[20] 	peer_id 	Your peer id.
	binary.BigEndian.PutUint64(buf[56:64], 0)                            // int64_t 	downloaded 	The number of byte you've downloaded in this session.
	binary.BigEndian.PutUint64(buf[64:72], uint64(metafile.Info.Length)) // int64_t 	left 	The number of bytes you have left to download until you're finished.
	binary.BigEndian.PutUint64(buf[72:80], 0)                            // int64_t 	uploaded 	The number of bytes you have uploaded in this session.
	binary.BigEndian.PutUint32(buf[80:84], 0)                            // int32_t 	event
	binary.BigEndian.PutUint32(buf[84:88], 0)                            // uint32_t 	ip 	Your ip address. Set to 0 if you want the tracker to use the sender of this UDP packet.
	binary.BigEndian.PutUint32(buf[88:92], uint32(transactionID))        // uint32_t 	key 	A unique key that is randomized by the client.
	binary.BigEndian.PutUint32(buf[92:96], uint32(peersRequested))       // int32_t 	num_want 	The maximum number of peers you want in the reply. Use -1 for default.
	binary.BigEndian.PutUint16(buf[96:98], 9999)                         // uint16_t 	port 	The port you're listening on.
	binary.BigEndian.PutUint16(buf[98:100], 0)                           // uint16_t 	extensions

	_, err = conn.Write(buf)
	if err != nil {
		return nil, err
	}

	buf = make([]byte, 100+peersRequested*6)
	readed, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	//action = binary.BigEndian.Uint32(buf[:4])
	//transaction_id = binary.BigEndian.Uint32(buf[4:8])
	interval := binary.BigEndian.Uint32(buf[8:12])
	// leechers := binary.BigEndian.Uint32(buf[12:16])
	// seeders := binary.BigEndian.Uint32(buf[16:20])

	peerData := buf[20:readed]

	peers := make([]models.Peer, 0)

	u.interval = int(interval)

	for {
		addr := models.Addr{}
		err = addr.ReadFromBytes(peerData[:6])
		if err != nil {
			return nil, err
		}

		peerData = peerData[6:]

		peer := models.Peer{
			Addr:         addr,
			HavePieces:   make(map[int]struct{}),
			PiecesWanted: len(metafile.Info.PiecesHashes),
		}

		peers = append(peers, peer)
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
