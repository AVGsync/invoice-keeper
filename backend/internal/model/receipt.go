package model

import (
	"errors"
	"time"
)

const (
	ReceiptKindFile = "file"
	ReceiptKindLink = "link"
)

var (
	ErrReceiptNotFound    = errors.New("receipt not found")
	ErrReceiptTooLarge    = errors.New("receipt file too large")
	ErrReceiptUnsupported = errors.New("receipt file type is not supported")
	ErrReceiptLimit       = errors.New("too many receipts for this payment")
)

// Receipt — чек к периоду платежа: файл на диске или внешняя ссылка.
type Receipt struct {
	ID             string
	PaymentID      string
	Kind           string
	Title          string
	URL            string
	FileName       string
	ContentType    string
	SizeBytes      int64
	UploadedBy     string
	UploadedByName string
	CreatedAt      time.Time
}
