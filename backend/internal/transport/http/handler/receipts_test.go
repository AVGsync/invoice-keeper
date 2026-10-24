package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"

	"github.com/go-chi/chi/v5"
)

type stubReceipts struct {
	gotTitle, gotName string
	gotBody           []byte
	err               error
}

func (s *stubReceipts) List(context.Context, string, service.PaymentActor) ([]model.Receipt, error) {
	return nil, s.err
}
func (s *stubReceipts) AttachFile(_ context.Context, _ service.PaymentActor, _, title, name string, body io.Reader) (model.Receipt, error) {
	s.gotTitle, s.gotName = title, name
	s.gotBody, _ = io.ReadAll(body)
	return model.Receipt{ID: "r1", Kind: "file", FileName: name, CreatedAt: time.Now()}, s.err
}
func (s *stubReceipts) AttachLink(_ context.Context, _ service.PaymentActor, _, title, raw string) (model.Receipt, error) {
	return model.Receipt{ID: "r2", Kind: "link", URL: raw, Title: title}, s.err
}
func (s *stubReceipts) Delete(context.Context, service.PaymentActor, string) error { return s.err }
func (s *stubReceipts) Access(context.Context, service.PaymentActor, string) (string, string, *time.Time, error) {
	return "link", "https://x.y", nil, s.err
}
func (s *stubReceipts) OpenSigned(context.Context, string, int64, string) (model.Receipt, *os.File, error) {
	return model.Receipt{}, nil, s.err
}

const payID = "9a191dfa-5e71-47aa-bcb2-ed5e9c071830"

func receiptRouter(h *ReceiptHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/payment-occurrences/{id}/receipts", h.Upload())
	r.Post("/payment-occurrences/{id}/receipts/link", h.AddLink())
	r.Get("/payment-occurrences/{id}/receipts", h.List())
	return r
}

func multipartBody(t *testing.T, fields map[string]string, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if fileName != "" {
		fw, _ := mw.CreateFormFile("file", fileName)
		fw.Write(content)
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestReceiptUpload(t *testing.T) {
	stub := &stubReceipts{}
	srv := receiptRouter(NewReceiptHandler(stub))

	body, ct := multipartBody(t, map[string]string{"title": "из формы"}, "c.pdf", []byte("%PDF-data"))
	req := withClaims(httptest.NewRequest(http.MethodPost, "/payment-occurrences/"+payID+"/receipts", body))
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || stub.gotName != "c.pdf" || string(stub.gotBody) != "%PDF-data" || stub.gotTitle != "из формы" {
		t.Fatalf("code=%d stub=%+v body=%s", rec.Code, stub, rec.Body.String())
	}

	body, ct = multipartBody(t, nil, "", nil)
	req = withClaims(httptest.NewRequest(http.MethodPost, "/payment-occurrences/"+payID+"/receipts", body))
	req.Header.Set("Content-Type", ct)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("no file: %d", rec.Code)
	}

	req = withClaims(httptest.NewRequest(http.MethodPost, "/payment-occurrences/"+payID+"/receipts", strings.NewReader("{}")))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("not multipart: %d", rec.Code)
	}
}

func TestReceiptErrorMapping(t *testing.T) {
	cases := map[error]int{
		model.ErrReceiptTooLarge:    http.StatusRequestEntityTooLarge,
		model.ErrReceiptUnsupported: http.StatusUnsupportedMediaType,
		model.ErrReceiptLimit:       http.StatusConflict,
		model.ErrPaymentForbidden:   http.StatusForbidden,
		model.ErrPaymentNotFound:    http.StatusNotFound,
		model.ErrReceiptNotFound:    http.StatusNotFound,
		service.ErrInvalidReceipt:   http.StatusBadRequest,
	}
	for err, want := range cases {
		srv := receiptRouter(NewReceiptHandler(&stubReceipts{err: err}))
		req := withClaims(httptest.NewRequest(http.MethodPost, "/payment-occurrences/"+payID+"/receipts/link", strings.NewReader(`{"url":"https://a.b"}`)))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%v: got %d, want %d", err, rec.Code, want)
		}
	}
}
