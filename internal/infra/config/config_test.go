package config

import (
	"os"
	"testing"
)

func TestPostgresDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "with password and user",
			config: Config{
				PostgresHost:     "localhost",
				PostgresPort:     "5432",
				PostgresDatabase: "test_db",
				PostgresUser:     "postgres",
				PostgresPassword: "secret",
				PostgresSSLMode:  "disable",
			},
			expected: "host=localhost port=5432 dbname=test_db sslmode=disable user=postgres password=secret",
		},
		{
			name: "without password",
			config: Config{
				PostgresHost:     "localhost",
				PostgresPort:     "5432",
				PostgresDatabase: "test_db",
				PostgresUser:     "postgres",
				PostgresPassword: "",
				PostgresSSLMode:  "disable",
			},
			expected: "host=localhost port=5432 dbname=test_db sslmode=disable user=postgres",
		},
		{
			name: "without user and password",
			config: Config{
				PostgresHost:     "localhost",
				PostgresPort:     "5432",
				PostgresDatabase: "test_db",
				PostgresUser:     "",
				PostgresPassword: "",
				PostgresSSLMode:  "disable",
			},
			expected: "host=localhost port=5432 dbname=test_db sslmode=disable",
		},
		{
			name: "custom host and port with SSL",
			config: Config{
				PostgresHost:     "db.example.com",
				PostgresPort:     "5433",
				PostgresDatabase: "my_db",
				PostgresUser:     "admin",
				PostgresPassword: "pass123",
				PostgresSSLMode:  "require",
			},
			expected: "host=db.example.com port=5433 dbname=my_db sslmode=require user=admin password=pass123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.PostgresDSN()
			if result != tt.expected {
				t.Errorf("PostgresDSN() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	_ = os.Setenv("POSTGRES_HOST", "testhost")
	_ = os.Setenv("POSTGRES_PORT", "5433")
	_ = os.Setenv("POSTGRES_DATABASE", "testdb")
	_ = os.Setenv("POSTGRES_USER", "testuser")
	defer func() {
		_ = os.Unsetenv("POSTGRES_HOST")
		_ = os.Unsetenv("POSTGRES_PORT")
		_ = os.Unsetenv("POSTGRES_DATABASE")
		_ = os.Unsetenv("POSTGRES_USER")
	}()

	cfg = nil
	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if config.PostgresHost != "testhost" {
		t.Errorf("PostgresHost = %v, want testhost", config.PostgresHost)
	}
	if config.PostgresPort != "5433" {
		t.Errorf("PostgresPort = %v, want 5433", config.PostgresPort)
	}
	if config.PostgresDatabase != "testdb" {
		t.Errorf("PostgresDatabase = %v, want testdb", config.PostgresDatabase)
	}
	if config.PostgresUser != "testuser" {
		t.Errorf("PostgresUser = %v, want testuser", config.PostgresUser)
	}
}
