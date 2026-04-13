CREATE TABLE IF NOT EXISTS scheduled_reminders (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    activated_voucher_id VARCHAR NOT NULL UNIQUE,
    hema_id              VARCHAR NOT NULL,
    voucher_id           VARCHAR NOT NULL,
    program_id           VARCHAR NOT NULL,
    characteristic       VARCHAR NOT NULL,
    send_at              TIMESTAMPTZ NOT NULL,
    status               VARCHAR NOT NULL DEFAULT 'PENDING',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index covering only PENDING rows. Stays small over time as
-- SENT and CANCELLED rows accumulate and are never scanned by the poller.
CREATE INDEX IF NOT EXISTS idx_reminders_due
    ON scheduled_reminders (send_at, status)
    WHERE status = 'PENDING';
