package models

type MessageID uint8

const (
	ChokeMessageID MessageID = iota
	UnchokeMessageID
	InterestedMessageID
	NotInterestedMessageID
	HaveMessageID
	BitfieldMessageID
	RequestMessageID
	PieceMessageID
	CancelMessageID
)
