package apiserver

import (
	"io"
	"log/slog"
	"os"

	"github.com/AVGsync/invoice-keeper/backend/internal/filestore"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
)

// brokenStore подменяет хранилище, если каталог недоступен: остальное приложение работает, а запросы к чекам отвечают ошибкой.
type brokenStore struct{ err error }

func (b brokenStore) Put(string, io.Reader, int64) (int64, error) { return 0, b.err }
func (b brokenStore) Open(string) (*os.File, error)               { return nil, b.err }
func (b brokenStore) Remove(string) error                         { return b.err }

func (s *APIServer) newReceiptService() *service.ReceiptService {
	var store service.ReceiptStorage
	fs, err := filestore.New(s.config.ReceiptsDir)
	if err != nil {
		slog.Error("receipts: storage unavailable", "dir", s.config.ReceiptsDir, "error", err)
		store = brokenStore{err: err}
	} else {
		store = fs
	}
	return service.NewReceiptService(s.db.Receipt(), s.db.Payment(), store,
		s.config.JWTAccessSecret, int64(s.config.ReceiptsMaxMB)<<20)
}
