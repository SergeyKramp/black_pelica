package reminder_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"hema/ces/internal/config"
	"hema/ces/internal/db"
	"hema/ces/internal/reminder"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testPool *pgxpool.Pool

// TestMain spins up a PostgreSQL container once for the entire package,
// runs the schema migration, then executes all tests. The container is
// terminated when all tests complete.
func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("ces_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		panic("start postgres container: " + err.Error())
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("get connection string: " + err.Error())
	}

	testPool, err = db.Connect(ctx, config.DatabaseConfig{
		URL:  connStr,
		Pool: config.DatabasePoolConfig{MaxConns: 5, MinConns: 1},
	})
	if err != nil {
		panic("connect to test database: " + err.Error())
	}
	defer testPool.Close()

	if err := runMigrations(ctx, testPool); err != nil {
		panic("run migrations: " + err.Error())
	}

	os.Exit(m.Run())
}

// runMigrations executes all .sql files in the migrations directory in
// lexicographic order against the test database.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, filepath.Join("../../migrations", e.Name()))
		}
	}
	sort.Strings(files)

	for _, path := range files {
		sql, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return err
		}
	}
	return nil
}

// truncate clears the reminders table between tests to ensure isolation.
func truncate(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE scheduled_reminders")
	require.NoError(t, err)
}

// Given: a new voucher activation
// When: upserting a reminder
// Then: a PENDING reminder is created with the correct fields
func TestUpsert_NewReminder_CreatesPendingRow(t *testing.T) {
	truncate(t)
	store := reminder.NewPostgresStore(testPool)

	r := reminder.Reminder{
		ActivatedVoucherID: "voucher-1",
		HemaID:             "hema-1",
		VoucherID:          "278",
		ProgramID:          "nl",
		Characteristic:     "HEMA",
		SendAt:             time.Now().Add(24 * time.Hour),
	}

	err := store.Upsert(context.Background(), r)
	require.NoError(t, err)

	var status string
	err = testPool.QueryRow(context.Background(),
		"SELECT status FROM scheduled_reminders WHERE activated_voucher_id = $1",
		r.ActivatedVoucherID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", status)
}

// Given: the same VoucherActivated event delivered twice
// When: upserting the same reminder twice
// Then: only one row exists (idempotent)
func TestUpsert_DuplicateEvent_DoesNotCreateDuplicateRow(t *testing.T) {
	truncate(t)
	store := reminder.NewPostgresStore(testPool)

	r := reminder.Reminder{
		ActivatedVoucherID: "voucher-2",
		HemaID:             "hema-2",
		VoucherID:          "278",
		ProgramID:          "nl",
		Characteristic:     "HEMA",
		SendAt:             time.Now().Add(24 * time.Hour),
	}

	require.NoError(t, store.Upsert(context.Background(), r))
	require.NoError(t, store.Upsert(context.Background(), r))

	var count int
	err := testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM scheduled_reminders WHERE activated_voucher_id = $1",
		r.ActivatedVoucherID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// Given: a PENDING reminder and an incoming VoucherRedeemed event
// When: cancelling the reminder
// Then: the reminder status is updated to CANCELLED
func TestCancel_PendingReminder_SetsCancelledStatus(t *testing.T) {
	truncate(t)
	store := reminder.NewPostgresStore(testPool)

	r := reminder.Reminder{
		ActivatedVoucherID: "voucher-3",
		HemaID:             "hema-3",
		VoucherID:          "278",
		ProgramID:          "nl",
		Characteristic:     "HEMA",
		SendAt:             time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, store.Upsert(context.Background(), r))

	err := store.Cancel(context.Background(), r.ActivatedVoucherID)
	require.NoError(t, err)

	var status string
	err = testPool.QueryRow(context.Background(),
		"SELECT status FROM scheduled_reminders WHERE activated_voucher_id = $1",
		r.ActivatedVoucherID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "CANCELLED", status)
}

// Given: a SENT reminder and an incoming VoucherRedeemed event
// When: cancelling the reminder
// Then: the status remains SENT (cancel is a no-op on non-PENDING rows)
func TestCancel_SentReminder_DoesNotChangeStatus(t *testing.T) {
	truncate(t)

	_, err := testPool.Exec(context.Background(), `
		INSERT INTO scheduled_reminders
			(activated_voucher_id, hema_id, voucher_id, program_id, characteristic, send_at, status)
		VALUES ('voucher-4', 'hema-4', '278', 'nl', 'HEMA', now(), 'SENT')`,
	)
	require.NoError(t, err)

	store := reminder.NewPostgresStore(testPool)
	err = store.Cancel(context.Background(), "voucher-4")
	require.NoError(t, err)

	var status string
	err = testPool.QueryRow(context.Background(),
		"SELECT status FROM scheduled_reminders WHERE activated_voucher_id = $1",
		"voucher-4",
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "SENT", status)
}

// Given: a due PENDING reminder
// When: running a batch
// Then: process is called with the reminder and it is marked SENT
func TestRunBatch_DueReminder_CallsProcessAndMarksSent(t *testing.T) {
	truncate(t)
	store := reminder.NewPostgresStore(testPool)

	r := reminder.Reminder{
		ActivatedVoucherID: "voucher-5",
		HemaID:             "hema-5",
		VoucherID:          "278",
		ProgramID:          "nl",
		Characteristic:     "HEMA",
		SendAt:             time.Now().Add(-time.Minute),
	}
	require.NoError(t, store.Upsert(context.Background(), r))

	var processed []string
	err := store.ReminderBatch(context.Background(), 10, func(reminders []reminder.Reminder) error {
		for _, r := range reminders {
			processed = append(processed, r.ActivatedVoucherID)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, processed, r.ActivatedVoucherID)

	var status string
	err = testPool.QueryRow(context.Background(),
		"SELECT status FROM scheduled_reminders WHERE activated_voucher_id = $1",
		r.ActivatedVoucherID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "SENT", status)
}

// Given: a due PENDING reminder and a failing process function
// When: running a batch
// Then: the reminder remains PENDING for retry on the next tick
func TestRunBatch_ProcessFailure_LeaveReminderPending(t *testing.T) {
	truncate(t)
	store := reminder.NewPostgresStore(testPool)

	r := reminder.Reminder{
		ActivatedVoucherID: "voucher-6",
		HemaID:             "hema-6",
		VoucherID:          "278",
		ProgramID:          "nl",
		Characteristic:     "HEMA",
		SendAt:             time.Now().Add(-time.Minute),
	}
	require.NoError(t, store.Upsert(context.Background(), r))

	err := store.ReminderBatch(context.Background(), 10, func(_ []reminder.Reminder) error {
		return assert.AnError
	})
	require.Error(t, err)

	var status string
	err = testPool.QueryRow(context.Background(),
		"SELECT status FROM scheduled_reminders WHERE activated_voucher_id = $1",
		r.ActivatedVoucherID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", status)
}

// Given: a PENDING reminder not yet due
// When: running a batch
// Then: the reminder is not processed
func TestRunBatch_FutureReminder_IsNotProcessed(t *testing.T) {
	truncate(t)
	store := reminder.NewPostgresStore(testPool)

	r := reminder.Reminder{
		ActivatedVoucherID: "voucher-7",
		HemaID:             "hema-7",
		VoucherID:          "278",
		ProgramID:          "nl",
		Characteristic:     "HEMA",
		SendAt:             time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, store.Upsert(context.Background(), r))

	var processed []string
	err := store.ReminderBatch(context.Background(), 10, func(reminders []reminder.Reminder) error {
		for _, r := range reminders {
			processed = append(processed, r.ActivatedVoucherID)
		}
		return nil
	})
	require.NoError(t, err)
	assert.NotContains(t, processed, r.ActivatedVoucherID)
}
