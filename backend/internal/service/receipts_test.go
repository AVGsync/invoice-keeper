package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/filestore"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

func TestDetectReceiptType(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
	pdf := []byte("%PDF-1.7\n")
	heic := append([]byte{0, 0, 0, 24}, []byte("ftypheic")...)
	zip := []byte("PK\x03\x04\x14\x00\x00\x00")
	cases := []struct {
		name string
		head []byte
		file string
		want string
		err  bool
	}{
		{"png", png, "a.png", "image/png", false},
		{"jpeg with wrong ext", jpg, "scan.bin", "image/jpeg", false},
		{"pdf", pdf, "x.pdf", "application/pdf", false},
		{"heic", heic, "IMG.HEIC", "image/heic", false},
		{"txt", []byte("Чек №1\nИтого 100"), "c.txt", "text/plain", false},
		{"csv", []byte("a,b\n1,2\n"), "c.csv", "text/csv", false},
		{"docx", zip, "c.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", false},
		{"zip without office ext", zip, "c.zip", "", true},
		{"html", []byte("<!DOCTYPE html><html><script>alert(1)</script>"), "c.pdf", "", true},
		{"svg", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), "c.svg", "", true},
		{"text with exe ext", []byte("hello"), "c.exe", "", true},
		{"binary", []byte{0, 1, 2, 3, 4, 5}, "c.pdf", "", true},
	}
	for _, c := range cases {
		got, err := detectReceiptType(c.head, c.file)
		if (err != nil) != c.err || got != c.want {
			t.Errorf("%s: got %q err=%v, want %q err=%v", c.name, got, err, c.want, c.err)
		}
	}
}

func TestNormalizeReceiptLink(t *testing.T) {
	ok := map[string]string{
		"https://check.example.com/r/1?x=1": "https://check.example.com/r/1?x=1",
		"  example.com/receipt ":            "https://example.com/receipt",
		"HTTP://Example.com/a":              "http://Example.com/a",
	}
	for in, want := range ok {
		got, _, err := normalizeReceiptLink(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "javascript:alert(1)", "ftp://x.com/a", "https://user:pass@x.com/", "https://", "http://a b.com", "file:///etc/passwd", "https://x.com/" + strings.Repeat("a", 3000)} {
		if _, _, err := normalizeReceiptLink(in); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd": "passwd",
		`C:\dir\чек.pdf`:   "чек.pdf",
		"a\"b;c.pdf":       "a_b_c.pdf",
		"":                 "receipt",
		"..":               "receipt",
		"x\x00y.png":       "x_y.png",
	}
	for in, want := range cases {
		if got := sanitizeFileName(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	long := sanitizeFileName(strings.Repeat("я", 300) + ".pdf")
	if r := []rune(long); len(r) != maxReceiptFileNameLen || !strings.HasSuffix(long, ".pdf") {
		t.Errorf("long name: %d runes, %q", len(r), long)
	}
}

type memReceipts struct {
	items map[string]model.Receipt
}

func (m *memReceipts) Create(_ context.Context, r model.Receipt) (model.Receipt, error) {
	r.CreatedAt = time.Now()
	m.items[r.ID] = r
	return r, nil
}
func (m *memReceipts) GetByID(_ context.Context, id string) (model.Receipt, error) {
	r, ok := m.items[id]
	if !ok {
		return model.Receipt{}, model.ErrReceiptNotFound
	}
	return r, nil
}
func (m *memReceipts) ListByPayment(_ context.Context, pid string) ([]model.Receipt, error) {
	var out []model.Receipt
	for _, r := range m.items {
		if r.PaymentID == pid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memReceipts) CountByPayment(ctx context.Context, pid string) (int, error) {
	l, _ := m.ListByPayment(ctx, pid)
	return len(l), nil
}
func (m *memReceipts) Delete(_ context.Context, id string) error {
	if _, ok := m.items[id]; !ok {
		return model.ErrReceiptNotFound
	}
	delete(m.items, id)
	return nil
}

type fixedPayments struct{ ownerID string }

func (f fixedPayments) GetPaymentByID(id string, _ context.Context) (model.Payment, error) {
	if id == "missing" {
		return model.Payment{}, model.ErrPaymentNotFound
	}
	return model.Payment{ID: id, UserID: f.ownerID}, nil
}

func newTestReceiptService(t *testing.T, max int64) (*ReceiptService, *memReceipts) {
	t.Helper()
	store, err := filestore.New(filepath.Join(t.TempDir(), "r"))
	if err != nil {
		t.Fatal(err)
	}
	repo := &memReceipts{items: map[string]model.Receipt{}}
	return NewReceiptService(repo, fixedPayments{ownerID: "owner"}, store, "secret", max), repo
}

func TestReceiptFileLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestReceiptService(t, 1<<10)
	owner := PaymentActor{UserID: "owner"}
	admin := PaymentActor{UserID: "boss", IsAdmin: true}
	stranger := PaymentActor{UserID: "other"}
	pdf := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 100)...)

	if _, err := svc.AttachFile(ctx, stranger, "pay", "", "a.pdf", bytes.NewReader(pdf)); err != model.ErrPaymentForbidden {
		t.Fatalf("stranger upload: %v", err)
	}
	rc, err := svc.AttachFile(ctx, owner, "pay", "  Чек  ", "dir/чек.pdf", bytes.NewReader(pdf))
	if err != nil {
		t.Fatal(err)
	}
	if rc.Kind != "file" || rc.ContentType != "application/pdf" || rc.FileName != "чек.pdf" || rc.Title != "Чек" || rc.SizeBytes != int64(len(pdf)) {
		t.Fatalf("unexpected receipt: %+v", rc)
	}

	if _, err := svc.AttachFile(ctx, owner, "pay", "", "big.pdf", bytes.NewReader(append([]byte("%PDF-"), make([]byte, 2<<10)...))); err != model.ErrReceiptTooLarge {
		t.Fatalf("too large: %v", err)
	}
	if _, err := svc.AttachFile(ctx, owner, "pay", "", "x.html", strings.NewReader("<html>")); err != model.ErrReceiptUnsupported {
		t.Fatalf("html: %v", err)
	}
	if _, err := svc.AttachFile(ctx, owner, "pay", "", "empty.pdf", strings.NewReader("")); err == nil {
		t.Fatal("empty file accepted")
	}
	if len(repo.items) != 1 {
		t.Fatalf("rejected uploads must not create rows: %d", len(repo.items))
	}

	kind, target, exp, err := svc.Access(ctx, admin, rc.ID)
	if err != nil || kind != "file" || exp == nil || !strings.HasPrefix(target, "/api/v1/receipt-files/"+rc.ID+"?") {
		t.Fatalf("access: %q %q %v %v", kind, target, exp, err)
	}
	if _, _, _, err := svc.Access(ctx, stranger, rc.ID); err != model.ErrPaymentForbidden {
		t.Fatalf("stranger access: %v", err)
	}

	sig := svc.sign(rc.ID, exp.Unix())
	got, f, err := svc.OpenSigned(ctx, rc.ID, exp.Unix(), sig)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, len(pdf))
	f.Read(body)
	f.Close()
	if got.ID != rc.ID || !bytes.Equal(body, pdf) {
		t.Fatal("file content mismatch")
	}
	if _, _, err := svc.OpenSigned(ctx, rc.ID, exp.Unix(), sig+"x"); err != model.ErrReceiptNotFound {
		t.Fatalf("bad signature: %v", err)
	}
	if _, _, err := svc.OpenSigned(ctx, rc.ID, exp.Unix()+1, sig); err != model.ErrReceiptNotFound {
		t.Fatalf("tampered exp: %v", err)
	}
	past := time.Now().Add(-time.Minute).Unix()
	if _, _, err := svc.OpenSigned(ctx, rc.ID, past, svc.sign(rc.ID, past)); err != model.ErrReceiptNotFound {
		t.Fatalf("expired: %v", err)
	}

	if err := svc.Delete(ctx, stranger, rc.ID); err != model.ErrPaymentForbidden {
		t.Fatalf("stranger delete: %v", err)
	}
	if err := svc.Delete(ctx, owner, rc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(svc.store.(*filestore.Store).Dir(), rc.ID[:2], rc.ID)); !os.IsNotExist(err) {
		t.Fatalf("file must be removed: %v", err)
	}
}

func TestReceiptLinkAndLimit(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestReceiptService(t, 1<<10)
	owner := PaymentActor{UserID: "owner"}
	for i := 0; i < maxReceiptsPerPayment; i++ {
		if _, err := svc.AttachLink(ctx, owner, "pay", "", "https://example.com/r"); err != nil {
			t.Fatalf("link %d: %v", i, err)
		}
	}
	if _, err := svc.AttachLink(ctx, owner, "pay", "", "https://example.com/r"); err != model.ErrReceiptLimit {
		t.Fatalf("limit: %v", err)
	}
	if _, err := svc.AttachLink(ctx, owner, "missing", "", "https://example.com"); err != model.ErrPaymentNotFound {
		t.Fatalf("missing payment: %v", err)
	}
}
