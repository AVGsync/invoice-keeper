package response

import (
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// ReceiptResponse — чек: загруженный файл (`kind=file`) или ссылка (`kind=link`).
type ReceiptResponse struct {
	// Идентификатор чека.
	ID string `json:"id" format:"uuid" example:"5f0a1e5e-8c9b-4a3e-9d3b-0b8f6f5a2c11"`
	// Период платежа, к которому прикреплён чек.
	PaymentID string `json:"payment_id" format:"uuid" example:"9a191dfa-5e71-47aa-bcb2-ed5e9c071830"`
	// Тип: `file` — файл на сервере, `link` — внешняя ссылка.
	Kind string `json:"kind" enums:"file,link" example:"file"`
	// Подпись; пустая, если не задана.
	Title string `json:"title" example:"Чек за октябрь"`
	// Адрес ссылки (только для `kind=link`).
	URL string `json:"url,omitempty" example:"https://example.com/receipt/123"`
	// Имя файла (только для `kind=file`).
	FileName string `json:"file_name,omitempty" example:"check.pdf"`
	// MIME-тип файла, определённый по содержимому.
	ContentType string `json:"content_type,omitempty" example:"application/pdf"`
	// Размер файла в байтах.
	SizeBytes int64 `json:"size_bytes,omitempty" example:"48213"`
	// Кто прикрепил; пусто, если пользователь удалён.
	UploadedBy     string    `json:"uploaded_by,omitempty" format:"uuid"`
	UploadedByName string    `json:"uploaded_by_name,omitempty" example:"Ольга Иванова"`
	CreatedAt      time.Time `json:"created_at" example:"2026-10-24T18:20:00Z"`
}

func NewReceiptResponse(r model.Receipt) ReceiptResponse {
	return ReceiptResponse{
		ID: r.ID, PaymentID: r.PaymentID, Kind: r.Kind, Title: r.Title, URL: r.URL,
		FileName: r.FileName, ContentType: r.ContentType, SizeBytes: r.SizeBytes,
		UploadedBy: r.UploadedBy, UploadedByName: r.UploadedByName, CreatedAt: r.CreatedAt,
	}
}

// ReceiptListResponse — чеки периода, новые последними.
type ReceiptListResponse struct {
	Items []ReceiptResponse `json:"items"`
	Total int               `json:"total" example:"2"`
}

// ReceiptAccessResponse — адрес для открытия чека.
type ReceiptAccessResponse struct {
	// `file` или `link`.
	Kind string `json:"kind" enums:"file,link"`
	// Для `link` — сама ссылка. Для `file` — путь вида `/api/v1/receipt-files/{id}?exp=…&sig=…`,
	// открывается без авторизации до `expires_at`.
	URL string `json:"url" example:"/api/v1/receipt-files/5f0a1e5e-8c9b-4a3e-9d3b-0b8f6f5a2c11?exp=1792866000&sig=abc"`
	// Когда перестанет работать адрес файла; у ссылок не задано.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
