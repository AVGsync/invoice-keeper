package service

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/filestore"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/google/uuid"
)

const (
	maxReceiptsPerPayment = 20
	maxReceiptTitleLen    = 120
	maxReceiptFileNameLen = 120
	maxReceiptURLLen      = 2048
	receiptAccessTTL      = 5 * time.Minute
)

var ErrInvalidReceipt = errors.New("invalid receipt")

type ReceiptRepository interface {
	Create(ctx context.Context, r model.Receipt) (model.Receipt, error)
	GetByID(ctx context.Context, id string) (model.Receipt, error)
	ListByPayment(ctx context.Context, paymentID string) ([]model.Receipt, error)
	CountByPayment(ctx context.Context, paymentID string) (int, error)
	Delete(ctx context.Context, id string) error
}

type ReceiptStorage interface {
	Put(key string, r io.Reader, max int64) (int64, error)
	Open(key string) (*os.File, error)
	Remove(key string) error
}

type receiptPayments interface {
	GetPaymentByID(id string, ctx context.Context) (model.Payment, error)
}

// ReceiptService прикрепляет к периоду платежа чеки: файлы и ссылки.
// Видеть и менять чеки может ответственный за платёж или админ.
type ReceiptService struct {
	repo     ReceiptRepository
	payments receiptPayments
	store    ReceiptStorage
	maxBytes int64
	key      []byte
	now      func() time.Time
	audit    audit.Recorder
}

// NewReceiptService: secret подписывает временные адреса файлов, maxBytes — предел размера файла.
func NewReceiptService(repo ReceiptRepository, payments receiptPayments, store ReceiptStorage, secret string, maxBytes int64) *ReceiptService {
	sum := sha256.Sum256([]byte("receipts:" + secret))
	return &ReceiptService{
		repo: repo, payments: payments, store: store, maxBytes: maxBytes,
		key: sum[:], now: time.Now, audit: audit.Nop{},
	}
}

func (s *ReceiptService) SetAuditRecorder(r audit.Recorder) { s.audit = r }

func (s *ReceiptService) authorize(ctx context.Context, paymentID string, actor PaymentActor) (model.Payment, error) {
	p, err := s.payments.GetPaymentByID(paymentID, ctx)
	if err != nil {
		return model.Payment{}, err
	}
	if !actor.IsAdmin && p.UserID != actor.UserID {
		return model.Payment{}, model.ErrPaymentForbidden
	}
	return p, nil
}

func (s *ReceiptService) List(ctx context.Context, paymentID string, actor PaymentActor) ([]model.Receipt, error) {
	if _, err := s.authorize(ctx, paymentID, actor); err != nil {
		return nil, err
	}
	return s.repo.ListByPayment(ctx, paymentID)
}

// AttachFile сохраняет файл-чек. Тип определяется по содержимому, а не по расширению.
func (s *ReceiptService) AttachFile(ctx context.Context, actor PaymentActor, paymentID, title, fileName string, body io.Reader) (model.Receipt, error) {
	if _, err := s.authorize(ctx, paymentID, actor); err != nil {
		return model.Receipt{}, err
	}
	if err := s.checkLimit(ctx, paymentID); err != nil {
		return model.Receipt{}, err
	}

	br := bufio.NewReaderSize(body, 1024)
	head, _ := br.Peek(512)
	if len(head) == 0 {
		return model.Receipt{}, fmt.Errorf("%w: file is empty", ErrInvalidReceipt)
	}
	name := sanitizeFileName(fileName)
	contentType, err := detectReceiptType(head, name)
	if err != nil {
		return model.Receipt{}, err
	}

	id := uuid.NewString()
	size, err := s.store.Put(id, br, s.maxBytes)
	if errors.Is(err, filestore.ErrTooLarge) {
		return model.Receipt{}, model.ErrReceiptTooLarge
	}
	if err != nil {
		return model.Receipt{}, fmt.Errorf("store receipt: %w", err)
	}

	rc, err := s.repo.Create(ctx, model.Receipt{
		ID: id, PaymentID: paymentID, Kind: model.ReceiptKindFile, Title: cleanTitle(title),
		FileName: name, ContentType: contentType, SizeBytes: size, UploadedBy: actor.UserID,
	})
	if err != nil {
		if rerr := s.store.Remove(id); rerr != nil {
			slog.Error("receipts: remove orphan file", "id", id, "error", rerr)
		}
		return model.Receipt{}, err
	}
	s.audit.Record(ctx, audit.Entry{
		Action: audit.ActionReceiptAttach, EntityType: audit.EntityReceipt, EntityID: rc.ID,
		Payload: map[string]any{
			"occurrence_id": paymentID, "kind": rc.Kind, "file_name": rc.FileName,
			"content_type": rc.ContentType, "size_bytes": rc.SizeBytes,
		},
	})
	return rc, nil
}

// AttachLink сохраняет ссылку на чек (http или https).
func (s *ReceiptService) AttachLink(ctx context.Context, actor PaymentActor, paymentID, title, rawURL string) (model.Receipt, error) {
	if _, err := s.authorize(ctx, paymentID, actor); err != nil {
		return model.Receipt{}, err
	}
	link, host, err := normalizeReceiptLink(rawURL)
	if err != nil {
		return model.Receipt{}, err
	}
	if err := s.checkLimit(ctx, paymentID); err != nil {
		return model.Receipt{}, err
	}
	rc, err := s.repo.Create(ctx, model.Receipt{
		ID: uuid.NewString(), PaymentID: paymentID, Kind: model.ReceiptKindLink,
		Title: cleanTitle(title), URL: link, UploadedBy: actor.UserID,
	})
	if err != nil {
		return model.Receipt{}, err
	}
	// В журнал идёт только хост: в адресе могут быть токены.
	s.audit.Record(ctx, audit.Entry{
		Action: audit.ActionReceiptAttach, EntityType: audit.EntityReceipt, EntityID: rc.ID,
		Payload: map[string]any{"occurrence_id": paymentID, "kind": rc.Kind, "host": host},
	})
	return rc, nil
}

func (s *ReceiptService) Delete(ctx context.Context, actor PaymentActor, id string) error {
	rc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.authorize(ctx, rc.PaymentID, actor); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if rc.Kind == model.ReceiptKindFile {
		if err := s.store.Remove(rc.ID); err != nil {
			slog.Error("receipts: remove file", "id", rc.ID, "error", err)
		}
	}
	s.audit.Record(ctx, audit.Entry{
		Action: audit.ActionReceiptDelete, EntityType: audit.EntityReceipt, EntityID: rc.ID,
		Payload: map[string]any{"occurrence_id": rc.PaymentID, "kind": rc.Kind, "file_name": rc.FileName},
	})
	return nil
}

// Access возвращает адрес для открытия чека: для ссылки — её саму, для файла — подписанный путь на 5 минут.
func (s *ReceiptService) Access(ctx context.Context, actor PaymentActor, id string) (kind, target string, expires *time.Time, err error) {
	rc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", "", nil, err
	}
	if _, err := s.authorize(ctx, rc.PaymentID, actor); err != nil {
		return "", "", nil, err
	}
	if rc.Kind == model.ReceiptKindLink {
		return rc.Kind, rc.URL, nil, nil
	}
	exp := s.now().Add(receiptAccessTTL)
	q := url.Values{"exp": {strconv.FormatInt(exp.Unix(), 10)}, "sig": {s.sign(rc.ID, exp.Unix())}}
	return rc.Kind, "/api/v1/receipt-files/" + rc.ID + "?" + q.Encode(), &exp, nil
}

// OpenSigned отдаёт файл по подписанному адресу без сессии.
func (s *ReceiptService) OpenSigned(ctx context.Context, id string, exp int64, sig string) (model.Receipt, *os.File, error) {
	if exp < s.now().Unix() || !hmac.Equal([]byte(sig), []byte(s.sign(id, exp))) {
		return model.Receipt{}, nil, model.ErrReceiptNotFound
	}
	rc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.Receipt{}, nil, err
	}
	if rc.Kind != model.ReceiptKindFile {
		return model.Receipt{}, nil, model.ErrReceiptNotFound
	}
	f, err := s.store.Open(rc.ID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Receipt{}, nil, model.ErrReceiptNotFound
		}
		return model.Receipt{}, nil, err
	}
	return rc, f, nil
}

func (s *ReceiptService) sign(id string, exp int64) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(id + "|" + strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *ReceiptService) checkLimit(ctx context.Context, paymentID string) error {
	n, err := s.repo.CountByPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if n >= maxReceiptsPerPayment {
		return model.ErrReceiptLimit
	}
	return nil
}

var officeTypes = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".odt":  "application/vnd.oasis.opendocument.text",
	".ods":  "application/vnd.oasis.opendocument.spreadsheet",
}

// detectReceiptType пропускает PDF, изображения, текст и офисные документы.
// HTML, XML и SVG отсекаются: они могли бы выполнить скрипт при открытии.
func detectReceiptType(head []byte, fileName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	sniffed := http.DetectContentType(head)
	switch {
	case strings.HasPrefix(sniffed, "application/pdf"):
		return "application/pdf", nil
	case sniffed == "image/jpeg", sniffed == "image/png", sniffed == "image/gif", sniffed == "image/webp":
		return sniffed, nil
	case isHEIC(head):
		return "image/heic", nil
	case strings.HasPrefix(sniffed, "text/plain"):
		switch ext {
		case ".txt":
			return "text/plain", nil
		case ".csv":
			return "text/csv", nil
		}
	case sniffed == "application/zip":
		if ct, ok := officeTypes[ext]; ok {
			return ct, nil
		}
	}
	return "", model.ErrReceiptUnsupported
}

func isHEIC(head []byte) bool {
	if len(head) < 12 || string(head[4:8]) != "ftyp" {
		return false
	}
	switch string(head[8:12]) {
	case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1":
		return true
	}
	return false
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`"/\;:*?<>|`, r) {
			r = '_'
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" || out == "." || out == ".." {
		return "receipt"
	}
	if r := []rune(out); len(r) > maxReceiptFileNameLen {
		ext := filepath.Ext(out)
		if len([]rune(ext)) > 10 {
			ext = ""
		}
		out = string(r[:maxReceiptFileNameLen-len([]rune(ext))]) + ext
	}
	return out
}

func cleanTitle(t string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(t) {
		if unicode.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if r := []rune(out); len(r) > maxReceiptTitleLen {
		out = string(r[:maxReceiptTitleLen])
	}
	return out
}

// normalizeReceiptLink принимает только http(s) без логина и пароля; без схемы подставляет https.
func normalizeReceiptLink(raw string) (link, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("%w: url is required", ErrInvalidReceipt)
	}
	if len(raw) > maxReceiptURLLen {
		return "", "", fmt.Errorf("%w: url is too long", ErrInvalidReceipt)
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, perr := url.Parse(raw)
	if perr != nil || u.Host == "" || strings.ContainsAny(raw, " \t\r\n") {
		return "", "", fmt.Errorf("%w: url is not valid", ErrInvalidReceipt)
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return "", "", fmt.Errorf("%w: only http and https links are allowed", ErrInvalidReceipt)
	}
	if u.User != nil {
		return "", "", fmt.Errorf("%w: url must not contain credentials", ErrInvalidReceipt)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return u.String(), u.Hostname(), nil
}
