package p2p

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"

	"github.com/WendelHime/gotorrent/internal/decoder"
	"github.com/WendelHime/gotorrent/internal/shared/models"
)

type P2PClient interface {
	Connect(address models.Addr) error
	Disconnect() error
	Handshake(hash models.Hash) error
	ReadMessage() (models.PeerMessage, error)
	WriteMessage(msg models.PeerMessage) error
}

type client struct {
	clientID string
	conn     net.Conn
}

func NewClient(clientID string) P2PClient {
	return &client{clientID: clientID, conn: nil}
}

func (c *client) Connect(address models.Addr) error {
	conn, err := net.Dial("tcp", address.String())
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *client) Disconnect() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

type handshake struct {
	Pstr     string
	InfoHash models.Hash
	PeerID   string
}

// handshake request to bytes
func (h handshake) Bytes() []byte {
	buf := make([]byte, 1)
	buf[0] = 19 // length of the protocol
	buf = append(buf, []byte("BitTorrent protocol")...)
	buf = append(buf, make([]byte, 8)...) // eight reserved bytes
	buf = append(buf, h.InfoHash.Hash...)
	buf = append(buf, []byte(h.PeerID)...) // peer id
	return buf
}

func (c *client) Handshake(hash models.Hash) error {
	req := handshake{
		Pstr:     "BitTorrent protocol",
		InfoHash: hash,
		PeerID:   c.clientID,
	}

	_, err := c.conn.Write(req.Bytes())
	if err != nil {
		return err
	}

	resp, err := decoder.ReadBytes(c.conn, 68)
	if err != nil {
		return err
	}

	_, err = decodeHandshake(resp)
	if err != nil {
		return err
	}

	return nil
}
func decodeHandshake(buf []byte) (handshake, error) {
	if len(buf) != 68 {
		return handshake{}, errors.New("invalid handshake")
	}

	return handshake{
		InfoHash: models.Hash{Hash: buf[28:48]},
		PeerID:   hex.EncodeToString(buf[48:]),
	}, nil
}

func (c *client) WriteMessage(msg models.PeerMessage) error {
	fmt.Printf("Sending message %+v\n", msg)
	buf := make([]byte, 5)
	binary.BigEndian.PutUint32(buf, uint32(msg.Length))
	buf[4] = byte(msg.ID)
	buf = append(buf, msg.Payload...)
	_, err := c.conn.Write(buf)
	return err
}

func (c *client) ReadMessage() (models.PeerMessage, error) {
	msgLengthBuff, err := decoder.ReadBytes(c.conn, 4)
	if err != nil {
		return models.PeerMessage{}, err
	}

	msgLength := int(binary.BigEndian.Uint32(msgLengthBuff))

	messageID, err := decoder.ReadBytes(c.conn, 1)
	if err != nil {
		return models.PeerMessage{}, err
	}

	payload := make([]byte, 0)
	if msgLength > 1 {
		payload, err = decoder.ReadBytes(c.conn, msgLength-1)
		if err != nil {
			return models.PeerMessage{}, err
		}
	}

	return models.PeerMessage{
		ID:      models.MessageID(messageID[0]),
		Payload: payload,
		Length:  int(msgLength),
	}, nil
}
