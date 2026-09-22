package experiment

import (
	"testing"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{Endpoint: "test", DurationSeconds: 60, RequestsPerSecond: 100, Workers: 10}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := (Config{Endpoint: "slow", DurationSeconds: 1, RequestsPerSecond: 1, Workers: 1}); err.Validate() != nil {
		t.Fatalf("minimal config rejected: %v", err)
	}

	tests := []struct {
		name string
		cfg  Config
	}{
		{"bad endpoint", Config{Endpoint: "fast", DurationSeconds: 10, RequestsPerSecond: 10, Workers: 1}},
		{"empty endpoint", Config{DurationSeconds: 10, RequestsPerSecond: 10, Workers: 1}},
		{"zero duration", Config{Endpoint: "test", RequestsPerSecond: 10, Workers: 1}},
		{"duration over cap", Config{Endpoint: "test", DurationSeconds: MaxDurationSeconds + 1, RequestsPerSecond: 10, Workers: 1}},
		{"zero rps", Config{Endpoint: "test", DurationSeconds: 10, Workers: 1}},
		{"rps over cap", Config{Endpoint: "test", DurationSeconds: 10, RequestsPerSecond: MaxRequestsPerSec + 1, Workers: 1}},
		{"zero workers", Config{Endpoint: "test", DurationSeconds: 10, RequestsPerSecond: 10}},
		{"workers over cap", Config{Endpoint: "test", DurationSeconds: 10, RequestsPerSecond: 10, Workers: MaxWorkers + 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatal("want validation error, got nil")
			}
		})
	}
}
