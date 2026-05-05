package main

import (
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

func TestParsePasswordType(t *testing.T) {
	tests := map[string]sidecarv1.PasswordType{
		"a":         sidecarv1.PasswordType_PASSWORD_TYPE_LOWERCASE,
		"A":         sidecarv1.PasswordType_PASSWORD_TYPE_UPPERCASE,
		"digits":    sidecarv1.PasswordType_PASSWORD_TYPE_DIGIT,
		"@":         sidecarv1.PasswordType_PASSWORD_TYPE_SYMBOL,
		"h":         sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER,
		"H":         sidecarv1.PasswordType_PASSWORD_TYPE_HEX_UPPER,
		"uuid-v7":   sidecarv1.PasswordType_PASSWORD_TYPE_UUID_V7,
		"hex_lower": sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER,
	}

	for input, want := range tests {
		got, err := parsePasswordType(input)
		if err != nil {
			t.Fatalf("parsePasswordType(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parsePasswordType(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestParsePasswordTypeRejectsUnknown(t *testing.T) {
	if _, err := parsePasswordType("unknown"); err == nil {
		t.Fatal("parsePasswordType returned nil error for unknown type")
	}
}
