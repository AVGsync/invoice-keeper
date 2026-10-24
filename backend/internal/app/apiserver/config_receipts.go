package apiserver

import "fmt"

// ReceiptsConfig — хранилище чеков платежей.
type ReceiptsConfig struct {
	// Каталог для загруженных файлов; в compose это отдельный том.
	ReceiptsDir string `envconfig:"RECEIPTS_DIR" default:"./data/receipts"`
	// Максимальный размер файла чека, МБ.
	ReceiptsMaxMB int `envconfig:"RECEIPTS_MAX_MB" default:"10"`
}

func (c *Config) validateReceipts() error {
	if c.ReceiptsDir == "" {
		c.ReceiptsDir = "./data/receipts"
	}
	if c.ReceiptsMaxMB == 0 {
		c.ReceiptsMaxMB = 10
	}
	if c.ReceiptsMaxMB < 1 || c.ReceiptsMaxMB > 50 {
		return fmt.Errorf("RECEIPTS_MAX_MB must be between 1 and 50")
	}
	return nil
}
