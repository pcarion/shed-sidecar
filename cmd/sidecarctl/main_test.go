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

func TestParseKeyValueConfType(t *testing.T) {
	tests := map[string]sidecarv1.KeyValueConfType{
		"space": sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_SPACE,
		"equal": sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		"=":     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		"colon": sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_COLON,
		":":     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_COLON,
	}

	for input, want := range tests {
		got, err := parseKeyValueConfType(input)
		if err != nil {
			t.Fatalf("parseKeyValueConfType(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseKeyValueConfType(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestParseKeyValueValueType(t *testing.T) {
	tests := map[string]sidecarv1.KeyValueValueType{
		"string": sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING,
		"number": sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER,
	}

	for input, want := range tests {
		got, err := parseKeyValueValueType(input)
		if err != nil {
			t.Fatalf("parseKeyValueValueType(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseKeyValueValueType(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestSplitKeyValueArg(t *testing.T) {
	key, value, err := splitKeyValueArg("name=db=main")
	if err != nil {
		t.Fatalf("splitKeyValueArg returned error: %v", err)
	}
	if key != "name" || value != "db=main" {
		t.Fatalf("splitKeyValueArg = %q, %q; want name, db=main", key, value)
	}
}

func TestSplitKeyValueArgRejectsInvalid(t *testing.T) {
	if _, _, err := splitKeyValueArg("missing"); err == nil {
		t.Fatal("splitKeyValueArg returned nil error for missing separator")
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

func TestPrintDockerStatus(t *testing.T) {
	var out bytes.Buffer
	printDockerStatus(&out, &sidecarv1.DockerStatusResponse{
		Containers: []*sidecarv1.ContainerStatus{
			{Name: "app", State: "running", Status: "Up 2 hours", Image: "postgres:16", Id: "abcdef012345"},
		},
	})

	got := out.String()
	if got != "NAME  STATE    STATUS      IMAGE        ID\napp   running  Up 2 hours  postgres:16  abcdef012345\n" {
		t.Fatalf("unexpected table:\n%s", got)
	}
}
