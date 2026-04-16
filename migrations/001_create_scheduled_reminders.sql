-- reminder_offsets is the single source of truth for how many days before
-- voucher expiry a reminder should be sent, keyed by voucher characteristic.
-- Ideally, these values can be configurable though a UI.
CREATE TABLE reminder_offsets (
    characteristic VARCHAR PRIMARY KEY,
    offset_days    INT NOT NULL CHECK (offset_days >= 0)
);

-- Some default reminder offsets for HEMA, PREMIUM, and GIFT vouchers.
INSERT INTO reminder_offsets (characteristic, offset_days) VALUES
    ('HEMA',    3),
    ('PREMIUM', 5),
    ('GIFT',    1);

-- scheduled_reminders stores reminders that have been scheduled to be sent to
-- voucher holders.
CREATE TABLE scheduled_reminders (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    activated_voucher_id VARCHAR     NOT NULL UNIQUE,
    hema_id              VARCHAR     NOT NULL,
    voucher_id           VARCHAR     NOT NULL,
    program_id           VARCHAR     NOT NULL,
    characteristic       VARCHAR     NOT NULL REFERENCES reminder_offsets(characteristic),
    valid_until          TIMESTAMPTZ NOT NULL,
    status               VARCHAR     NOT NULL DEFAULT 'PENDING'
                             CHECK (status IN ('PENDING', 'SENT', 'CANCELLED')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index covering only PENDING rows. Stays small over time as SENT and
-- CANCELLED rows accumulate and are never scanned by the scheduler.
CREATE INDEX idx_reminders_pending
    ON scheduled_reminders (valid_until)
    WHERE status = 'PENDING';
