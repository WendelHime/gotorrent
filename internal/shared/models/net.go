package models

import (
	"encoding/binary"
	"errors"
	"net"
	"strconv"
)

type Addr struct {
	IP   net.IP
	Port uint16
}

func (a *Addr) String() string {
	return net.JoinHostPort(a.IP.String(), strconv.Itoa(int(a.Port)))
}

var ErrInvalidAddr = errors.New("invalid address")

func (a *Addr) ReadFromBytes(b []byte) error {
	if len(b) != 6 {
		return ErrInvalidAddr
	}

	a.IP = net.IP(b[:4])
	a.Port = binary.BigEndian.Uint16(b[4:])

	return nil
}
