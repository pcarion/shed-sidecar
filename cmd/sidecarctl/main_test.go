package main

import (
	"bytes"
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

func TestParsePgHbaType(t *testing.T) {
	tests := map[string]sidecarv1.PgHbaType{
		"local": sidecarv1.PgHbaType_PG_HBA_TYPE_LOCAL,
		"host":  sidecarv1.PgHbaType_PG_HBA_TYPE_HOST,
	}

	for input, want := range tests {
		got, err := parsePgHbaType(input)
		if err != nil {
			t.Fatalf("parsePgHbaType(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parsePgHbaType(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestParsePgHbaTypeRejectsUnknown(t *testing.T) {
	if _, err := parsePgHbaType("unknown"); err == nil {
		t.Fatal("parsePgHbaType returned nil error for unknown type")
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("app, migrator,, readonly ")
	want := []string{"app", "migrator", "readonly"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrintPasswordList(t *testing.T) {
	var out bytes.Buffer
	printPasswordList(&out, &sidecarv1.PasswordListResponse{
		Services: []*sidecarv1.PasswordService{
			{
				ServiceName: "svc",
				Passwords: []*sidecarv1.PasswordEntry{
					{Name: "admin", Password: "secret"},
				},
			},
		},
	})

	got := out.String()
	if got != "SERVICE  NAME   PASSWORD\nsvc      admin  secret\n" {
		t.Fatalf("unexpected table:\n%s", got)
	}
}

func TestPrintNetworkList(t *testing.T) {
	var out bytes.Buffer
	printNetworkList(&out, &sidecarv1.NetworkListResponse{
		Networks: []*sidecarv1.NetworkEntry{
			{ServiceName: "svc", Name: "http", Port: 20000},
		},
	})

	got := out.String()
	if got != "SERVICE  NAME  PORT\nsvc      http  20000\n" {
		t.Fatalf("unexpected table:\n%s", got)
	}
}

func TestPrintParamList(t *testing.T) {
	var out bytes.Buffer
	printParamList(&out, &sidecarv1.ParamListResponse{
		Services: []*sidecarv1.ParamService{
			{
				ServiceName: "svc",
				Params: []*sidecarv1.ParamEntry{
					{Name: "api-url", Value: "https://example.test"},
				},
			},
		},
	})

	got := out.String()
	if got != "SERVICE  NAME     VALUE\nsvc      api-url  https://example.test\n" {
		t.Fatalf("unexpected table:\n%s", got)
	}
}
