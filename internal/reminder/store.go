package reminder

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL implementation of both Store and SchedulerStore.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgresStore backed by the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Upsert inserts a new PENDING reminder or updates an existing one if it is still PENDING.
// If the reminder is already SENT or CANCELLED the upsert is a no-op, preventing
// a duplicate Kafka event from re-scheduling an already processed reminder.
func (s *PostgresStore) Upsert(ctx context.Context, reminder Reminder) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO scheduled_reminders
			(activated_voucher_id, hema_id, voucher_id, program_id, characteristic, send_at, status, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, 'PENDING', now())
		ON CONFLICT (activated_voucher_id) DO UPDATE
			SET send_at    = EXCLUDED.send_at,
			    status     = 'PENDING',
			    updated_at = now()
		WHERE scheduled_reminders.status = 'PENDING'`,
		reminder.ActivatedVoucherID, reminder.HemaID, reminder.VoucherID, reminder.ProgramID, reminder.Characteristic, reminder.SendAt,
	)
	if err != nil {
		return fmt.Errorf("upsert reminder %s: %w", reminder.ActivatedVoucherID, err)
	}
	return nil
}

// Cancel marks a PENDING reminder as CANCELLED. It is safe to call if the reminder
// does not exist or has already been SENT or CANCELLED.
func (s *PostgresStore) Cancel(ctx context.Context, activatedVoucherID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scheduled_reminders
		SET    status     = 'CANCELLED',
		       updated_at = now()
		WHERE  activated_voucher_id = $1
		AND    status = 'PENDING'`,
		activatedVoucherID,
	)
	if err != nil {
		return fmt.Errorf("cancel reminder %s: %w", activatedVoucherID, err)
	}
	return nil
}

// RunBatch opens a transaction, selects up to limit due PENDING reminders using
// SELECT FOR UPDATE SKIP LOCKED so concurrent workers each receive a disjoint set
func (s *PostgresStore) RunBatch(ctx context.Context, limit int, process func([]Reminder) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `
		SELECT id, activated_voucher_id, hema_id, voucher_id, program_id, characteristic, send_at
		FROM   scheduled_reminders
		WHERE  status = 'PENDING'
		AND    send_at <= now()
		ORDER  BY send_at
		LIMIT  $1
		FOR UPDATE SKIP LOCKED`,
		limit,
	)
	if err != nil {
		return fmt.Errorf("query due reminders: %w", err)
	}

	reminders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Reminder, error) {
		var r Reminder
		err := row.Scan(
			&r.ID, &r.ActivatedVoucherID, &r.HemaID,
			&r.VoucherID, &r.ProgramID, &r.Characteristic, &r.SendAt,
		)
		return r, err
	})
	if err != nil {
		return fmt.Errorf("scan reminders: %w", err)
	}

	if len(reminders) == 0 {
		return nil
	}

	if err := process(reminders); err != nil {
		return err
	}

	ids := make([]string, len(reminders))
	for i, r := range reminders {
		ids[i] = r.ID
	}

	if _, err := tx.Exec(ctx, `
		UPDATE scheduled_reminders
		SET    status     = 'SENT',
		       updated_at = now()
		WHERE  id = ANY($1)`,
		ids,
	); err != nil {
		return fmt.Errorf("mark batch sent: %w", err)
	}

	return tx.Commit(ctx)
}
