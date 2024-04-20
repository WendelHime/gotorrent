package models

type MessageID uint8

const (
	MessageIDChoke MessageID = iota
	MessageIDUnchoke
	MessageIDInterested
	MessageIDNotInterested
	MessageIDHave
	MessageIDBitfield
	MessageIDRequest
	MessageIDPiece
	MessageIDCancel
)
