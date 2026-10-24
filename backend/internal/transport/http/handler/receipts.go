package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

type ReceiptUseCase interface {
	List(ctx context.Context, paymentID string, actor service.PaymentActor) ([]model.Receipt, error)
	AttachFile(ctx context.Context, actor service.PaymentActor, paymentID, title, fileName string, body io.Reader) (model.Receipt, error)
	AttachLink(ctx context.Context, actor service.PaymentActor, paymentID, title, rawURL string) (model.Receipt, error)
	Delete(ctx context.Context, actor service.PaymentActor, id string) error
	Access(ctx context.Context, actor service.PaymentActor, id string) (kind, target string, expires *time.Time, err error)
	OpenSigned(ctx context.Context, id string, exp int64, sig string) (model.Receipt, *os.File, error)
}

type ReceiptHandler struct {
	receipts ReceiptUseCase
}

func NewReceiptHandler(receipts ReceiptUseCase) *ReceiptHandler {
	return &ReceiptHandler{receipts: receipts}
}

func writeReceiptError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	switch {
	case errors.Is(err, model.ErrReceiptNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, model.ErrReceiptTooLarge), errors.As(err, &tooBig):
		http.Error(w, "file is too large", http.StatusRequestEntityTooLarge)
	case errors.Is(err, model.ErrReceiptUnsupported):
		http.Error(w, "unsupported file type: allowed pdf, images, txt, csv, docx, xlsx, odt, ods", http.StatusUnsupportedMediaType)
	case errors.Is(err, model.ErrReceiptLimit):
		http.Error(w, "too many receipts for this payment", http.StatusConflict)
	case errors.Is(err, service.ErrInvalidReceipt):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		writePaymentError(w, err)
	}
}

func receiptActor(r *http.Request) (service.PaymentActor, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		return service.PaymentActor{}, false
	}
	return service.PaymentActor{UserID: claims.UserID, IsAdmin: claims.Role == "admin"}, true
}

// ListReceipts godoc
// @Summary      Чеки платежа
// @Description  Файлы и ссылки, прикреплённые к периоду платежа. Доступно ответственному за платёж и администратору.
// @Tags         Receipts
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "UUID периода платежа"
// @Success      200 {object} response.ReceiptListResponse
// @Failure      400 {string} string "Некорректный id"
// @Failure      403 {string} string "Недостаточно прав"
// @Failure      404 {string} string "Платёж не найден"
// @Router       /payment-occurrences/{id}/receipts [get]
func (h *ReceiptHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := receiptActor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}
		items, err := h.receipts.List(r.Context(), id, actor)
		if err != nil {
			writeReceiptError(w, err)
			return
		}
		out := response.ReceiptListResponse{Items: make([]response.ReceiptResponse, 0, len(items)), Total: len(items)}
		for _, it := range items {
			out.Items = append(out.Items, response.NewReceiptResponse(it))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// UploadReceipt godoc
// @Summary      Загрузить файл чека
// @Description  `multipart/form-data` с полем `file`. Подпись — query-параметр `title` или поле формы `title`, если оно идёт раньше файла.
// @Description  Форматы определяются по содержимому: PDF, JPEG, PNG, GIF, WebP, HEIC, TXT, CSV, DOCX, XLSX, ODT, ODS.
// @Description  Максимальный размер задаёт `RECEIPTS_MAX_MB` (по умолчанию 10 МБ), не больше 20 чеков на платёж.
// @Tags         Receipts
// @Security     BearerAuth
// @Accept       mpfd
// @Produce      json
// @Param        id path string true "UUID периода платежа"
// @Param        title query string false "Подпись, до 120 символов"
// @Param        file formData file true "Файл чека"
// @Success      201 {object} response.ReceiptResponse
// @Failure      400 {string} string "Нет файла или пустой файл"
// @Failure      403 {string} string "Недостаточно прав"
// @Failure      404 {string} string "Платёж не найден"
// @Failure      409 {string} string "Достигнут лимит чеков"
// @Failure      413 {string} string "Файл слишком большой"
// @Failure      415 {string} string "Неподдерживаемый формат"
// @Router       /payment-occurrences/{id}/receipts [post]
func (h *ReceiptHandler) Upload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := receiptActor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}
		mr, err := r.MultipartReader()
		if err != nil {
			http.Error(w, "multipart/form-data expected", http.StatusBadRequest)
			return
		}

		title := r.URL.Query().Get("title")
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				http.Error(w, "field file is required", http.StatusBadRequest)
				return
			}
			if err != nil {
				writeReceiptError(w, err)
				return
			}
			switch part.FormName() {
			case "title":
				if b, _ := io.ReadAll(io.LimitReader(part, 512)); title == "" {
					title = string(b)
				}
			case "file":
				rc, err := h.receipts.AttachFile(r.Context(), actor, id, title, part.FileName(), part)
				if err != nil {
					writeReceiptError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, response.NewReceiptResponse(rc))
				return
			}
			part.Close()
		}
	}
}

// AddReceiptLink godoc
// @Summary      Прикрепить ссылку на чек
// @Description  Принимаются только http и https; без схемы подставляется https. Логин и пароль в адресе запрещены.
// @Tags         Receipts
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path string true "UUID периода платежа"
// @Param        request body request.AddReceiptLinkRequest true "Ссылка и подпись"
// @Success      201 {object} response.ReceiptResponse
// @Failure      400 {string} string "Некорректная ссылка"
// @Failure      403 {string} string "Недостаточно прав"
// @Failure      404 {string} string "Платёж не найден"
// @Failure      409 {string} string "Достигнут лимит чеков"
// @Router       /payment-occurrences/{id}/receipts/link [post]
func (h *ReceiptHandler) AddLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := receiptActor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}
		var req request.AddReceiptLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		rc, err := h.receipts.AttachLink(r.Context(), actor, id, req.Title, req.URL)
		if err != nil {
			writeReceiptError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, response.NewReceiptResponse(rc))
	}
}

// DeleteReceipt godoc
// @Summary      Удалить чек
// @Description  Удаляет ссылку или файл вместе с содержимым. Доступно ответственному за платёж и администратору.
// @Tags         Receipts
// @Security     BearerAuth
// @Param        id path string true "UUID чека"
// @Success      204 "Чек удалён"
// @Failure      403 {string} string "Недостаточно прав"
// @Failure      404 {string} string "Чек не найден"
// @Router       /receipts/{id} [delete]
func (h *ReceiptHandler) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := receiptActor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}
		if err := h.receipts.Delete(r.Context(), actor, id); err != nil {
			writeReceiptError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ReceiptAccess godoc
// @Summary      Адрес для открытия чека
// @Description  Для ссылки возвращает её саму. Для файла — путь `/api/v1/receipt-files/{id}?exp=…&sig=…`,
// @Description  который открывается без авторизации (в браузере, в том числе внутри Telegram) в течение 5 минут.
// @Tags         Receipts
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "UUID чека"
// @Success      200 {object} response.ReceiptAccessResponse
// @Failure      403 {string} string "Недостаточно прав"
// @Failure      404 {string} string "Чек не найден"
// @Router       /receipts/{id}/access [get]
func (h *ReceiptHandler) Access() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := receiptActor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}
		kind, target, exp, err := h.receipts.Access(r.Context(), actor, id)
		if err != nil {
			writeReceiptError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response.ReceiptAccessResponse{Kind: kind, URL: target, ExpiresAt: exp})
	}
}

// ServeReceiptFile godoc
// @Summary      Скачать файл чека по подписанному адресу
// @Description  Адрес выдаёт `GET /receipts/{id}/access`; без сессии, действует 5 минут. PDF и изображения открываются в браузере, остальное скачивается.
// @Tags         Receipts
// @Param        id path string true "UUID чека"
// @Param        exp query int true "Время окончания, unix"
// @Param        sig query string true "Подпись"
// @Success      200 {file} file "Файл"
// @Failure      404 {string} string "Адрес недействителен или файла нет"
// @Router       /receipt-files/{id} [get]
func (h *ReceiptHandler) ServeFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exp, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		rc, f, err := h.receipts.OpenSigned(r.Context(), chi.URLParam(r, "id"), exp, r.URL.Query().Get("sig"))
		if err != nil {
			writeReceiptError(w, err)
			return
		}
		defer f.Close()

		disposition := "attachment"
		if rc.ContentType == "application/pdf" || (len(rc.ContentType) > 6 && rc.ContentType[:6] == "image/") {
			disposition = "inline"
		}
		hdr := w.Header()
		hdr.Set("Content-Type", rc.ContentType)
		hdr.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": rc.FileName}))
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
		hdr.Set("Cache-Control", "private, no-store")
		http.ServeContent(w, r, rc.FileName, rc.CreatedAt, f)
	}
}
