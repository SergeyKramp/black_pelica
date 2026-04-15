# Email Reminder Service

A Go service that schedules and sends voucher reminder emails before expiry, and cancels them when a voucher is redeemed.

## How it works

```
Kafka (VoucherActivated)  ──►  Consumer  ──►  PostgreSQL (PENDING)
Kafka (VoucherRedeemed)   ──►  Consumer  ──►  PostgreSQL (CANCELLED)

Scheduler (every tick)    ──►  PostgreSQL (SELECT FOR UPDATE SKIP LOCKED)
                          ──►  Selligent bulk email API
                          ──►  PostgreSQL (SENT)
```

**Consumer** — listens on a Kafka topic for two event types:

- `VoucherActivated` — schedules a reminder `N` days before `validUntil`, where `N` is configured per voucher characteristic
- `VoucherRedeemed` — cancels any pending reminder for that voucher

**Scheduler** — runs on a configurable interval, spawning multiple concurrent workers. Each worker claims a disjoint batch of due reminders via `SELECT FOR UPDATE SKIP LOCKED`, sends them in bulk to Selligent, and marks them `SENT`. The scheduler drains all due reminders per tick before stopping.

## Prerequisites

- Go 1.25+
- Docker (for local infrastructure)

## Running locally

**1. Start infrastructure**

```bash
docker compose up -d
```

This starts PostgreSQL (with migrations applied automatically) and Kafka.

**2. Start the mock Selligent server**

```bash
go run ./cmd/selligent-mock
```

Listens on `:8080`, logs every received email batch, and returns 200 OK.

**3. Start the CES server**

```bash
go run ./cmd/server
```

Flags:
| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.yaml` | Path to the YAML config file |
| `-debug` | `false` | Enable debug-level logging |

**4. Produce mock events**

```bash
go run ./cmd/producer
```

Sends a `VoucherActivated` event every 5 seconds with `validUntil = now + 1 minute`, so reminders are due immediately on the next scheduler tick. Every third tick also sends a `VoucherRedeemed` to test cancellation.

Flags:
| Flag | Default | Description |
|------|---------|-------------|
| `-broker` | `localhost:9092` | Kafka broker address |
| `-interval` | `5s` | Interval between produced events |

## Configuration

All settings live in `config.yaml`:

```yaml
database:
  url: postgres://user:pass@localhost:5432/ces?sslmode=disable
  pool:
    max_conns: 10
    min_conns: 2

kafka:
  brokers:
    - localhost:9092
  vouchers:
    topic: Hema.Loyalty.Voucher
    consumer_group: ces-voucher-reminders

selligent:
  base_url: http://localhost:8080
  api_key: your-api-key

# Days before expiry to send a reminder, per voucher type.
# Omit a characteristic or set to 0 to disable reminders for that type.
reminder_offsets:
  HEMA: 3
  PREMIUM: 5
  GIFT: 1

scheduler:
  interval_seconds: 3600
  worker_count: 5
  batch_size: 10
```

## Running tests

Unit and integration tests (integration tests require Docker for Testcontainers):

```bash
go test ./...
```
