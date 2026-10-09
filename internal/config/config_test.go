package config

import "testing"

func TestValidate(t *testing.T) {
	good := Config{DatabaseURL: "postgres://x", JWTSecret: "0123456789abcdef", Port: "8086"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := (Config{Port: "8086"}).Validate(); err == nil {
		t.Fatal("missing JWT_SECRET should fail")
	}
	if err := (Config{JWTSecret: "short", Port: "8086"}).Validate(); err == nil {
		t.Fatal("short JWT_SECRET should fail")
	}
	if err := (Config{JWTSecret: "0123456789abcdef", Port: "abc"}).Validate(); err == nil {
		t.Fatal("bad port should fail")
	}
}
