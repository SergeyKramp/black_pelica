package config_test

import (
	"strings"
	"testing"

	"hema/ces/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Given: a valid YAML config reader
// When: parsing the configuration
// Then: all fields are correctly populated
func TestParse_ValidYAML_ParsesAllFields(t *testing.T) {
	yaml := `
database:
  url: postgres://user:pass@localhost:5432/ces?sslmode=disable
kafka:
  brokers:
    - localhost:9092
  vouchers:
    topic: Hema.Loyalty.Voucher
    consumer_group: ces-voucher-reminders
selligent:
  base_url: https://api.selligent.example.com
  api_key: test-api-key
reminder_offsets:
  HEMA: 3
  PREMIUM: 5
  GIFT: 0
scheduler:
  interval_seconds: 60
`
	cfg, err := config.Parse(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@localhost:5432/ces?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, []string{"localhost:9092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "Hema.Loyalty.Voucher", cfg.Kafka.Vouchers.Topic)
	assert.Equal(t, "ces-voucher-reminders", cfg.Kafka.Vouchers.ConsumerGroup)
	assert.Equal(t, "https://api.selligent.example.com", cfg.Selligent.BaseURL)
	assert.Equal(t, "test-api-key", cfg.Selligent.APIKey)
	assert.Equal(t, 3, cfg.ReminderOffsets["HEMA"])
	assert.Equal(t, 5, cfg.ReminderOffsets["PREMIUM"])
	assert.Equal(t, 60, cfg.Scheduler.IntervalSeconds)
}

// Given: an invalid YAML reader
// When: parsing the configuration
// Then: an error is returned
func TestParse_InvalidYAML_ReturnsError(t *testing.T) {
	_, err := config.Parse(strings.NewReader("not: valid: yaml: ::"))
	require.Error(t, err)
}

// Given: a valid YAML config file on disk
// When: loading the configuration by file path
// Then: the config is returned without error
func TestLoad_ValidFile_ReturnsConfig(t *testing.T) {
	cfg, err := config.Load("testdata/config.yaml")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

// Given: a path to a file that does not exist
// When: loading the configuration
// Then: an error is returned
func TestLoad_MissingFile_ReturnsError(t *testing.T) {
	_, err := config.Load("testdata/nonexistent.yaml")
	require.Error(t, err)
}
