package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// ReceiptRepository хранит чеки к периодам платежей (таблица payment_receipts).
type ReceiptRepository struct {
	database *DB
}

const receiptSelect = `
	SELECT pr.id::text, pr.payment_id::text, pr.kind, pr.title,
	       COALESCE(pr.url, ''), COALESCE(pr.file_name, ''), COALESCE(pr.content_type, ''),
	       COALESCE(pr.size_bytes, 0), COALESCE(pr.uploaded_by::text, ''), COALESCE(u.full_name, ''),
	       pr.created_at
	FROM payment_receipts pr
	LEFT JOIN users u ON u.id = pr.uploaded_by`

func scanReceipt(row interface{ Scan(...any) error }) (model.Receipt, error) {
	var r model.Receipt
	err := row.Scan(&r.ID, &r.PaymentID, &r.Kind, &r.Title, &r.URL, &r.FileName, &r.ContentType,
		&r.SizeBytes, &r.UploadedBy, &r.UploadedByName, &r.CreatedAt)
	return r, err
}

// Create вставляет чек с заранее заданным id (он же ключ файла в хранилище).
func (r *ReceiptRepository) Create(ctx context.Context, rc model.Receipt) (model.Receipt, error) {
	const q = `
		INSERT INTO payment_receipts
			(id, payment_id, kind, title, url, file_name, content_type, size_bytes, uploaded_by)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''),
		        CASE WHEN $3 = 'file' THEN $8::bigint END, NULLIF($9, '')::uuid)
		RETURNING id::text`
	var id string
	err := r.database.db.QueryRowContext(ctx, q, rc.ID, rc.PaymentID, rc.Kind, rc.Title,
		rc.URL, rc.FileName, rc.ContentType, rc.SizeBytes, rc.UploadedBy).Scan(&id)
	if err != nil {
		return model.Receipt{}, fmt.Errorf("create receipt: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *ReceiptRepository) GetByID(ctx context.Context, id string) (model.Receipt, error) {
	rc, err := scanReceipt(r.database.db.QueryRowContext(ctx, receiptSelect+` WHERE pr.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Receipt{}, model.ErrReceiptNotFound
	}
	if err != nil {
		return model.Receipt{}, fmt.Errorf("get receipt: %w", err)
	}
	return rc, nil
}

func (r *ReceiptRepository) ListByPayment(ctx context.Context, paymentID string) ([]model.Receipt, error) {
	rows, err := r.database.db.QueryContext(ctx, receiptSelect+` WHERE pr.payment_id = $1 ORDER BY pr.created_at, pr.id`, paymentID)
	if err != nil {
		return nil, fmt.Errorf("list receipts: %w", err)
	}
	defer rows.Close()
	out := []model.Receipt{}
	for rows.Next() {
		rc, err := scanReceipt(rows)
		if err != nil {
			return nil, fmt.Errorf("scan receipt: %w", err)
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

func (r *ReceiptRepository) CountByPayment(ctx context.Context, paymentID string) (int, error) {
	var n int
	err := r.database.db.QueryRowContext(ctx, `SELECT count(*) FROM payment_receipts WHERE payment_id = $1`, paymentID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count receipts: %w", err)
	}
	return n, nil
}

func (r *ReceiptRepository) Delete(ctx context.Context, id string) error {
	res, err := r.database.db.ExecContext(ctx, `DELETE FROM payment_receipts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete receipt: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrReceiptNotFound
	}
	return nil
}
