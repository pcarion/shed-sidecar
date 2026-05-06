package pghba

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

func Configure(req *sidecarv1.ConfigurePgHbaConfRequest, configDir string) (bool, error) {
	filePath := req.GetFilePath()
	if filePath == "" {
		return false, fmt.Errorf("pg_hba.conf file path is required")
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(configDir, filePath)
	}

	columns := columns(req)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("read pg_hba.conf %q: %w", filePath, err)
	}
	if containsColumns(data, columns) {
		return false, nil
	}
	if err := backup(filePath, data, filepath.Join(configDir, "archive")); err != nil {
		return false, err
	}
	if err := appendLine(filePath, data, strings.Join(columns, "\t")); err != nil {
		return false, err
	}
	return true, nil
}

func columns(req *sidecarv1.ConfigurePgHbaConfRequest) []string {
	out := []string{
		typeColumn(req.GetType()),
		req.GetDatabase(),
		strings.Join(req.GetUsers(), ","),
	}
	if req.GetType() != sidecarv1.PgHbaType_PG_HBA_TYPE_LOCAL && req.Address != nil {
		out = append(out, req.GetAddress())
	}
	if req.GetMethod() != "" {
		out = append(out, req.GetMethod())
	}
	if req.Options != nil {
		out = append(out, strings.Fields(req.GetOptions())...)
	}
	return out
}

func typeColumn(value sidecarv1.PgHbaType) string {
	switch value {
	case sidecarv1.PgHbaType_PG_HBA_TYPE_LOCAL:
		return "local"
	case sidecarv1.PgHbaType_PG_HBA_TYPE_HOST:
		return "host"
	default:
		return strings.ToLower(strings.TrimPrefix(value.String(), "PG_HBA_TYPE_"))
	}
}

func containsColumns(data []byte, want []string) bool {
	for _, raw := range bytes.Split(data, []byte("\n")) {
		line := strings.TrimSpace(stripComment(string(raw)))
		if line == "" {
			continue
		}
		if equalColumns(strings.Fields(line), want) {
			return true
		}
	}
	return false
}

func stripComment(line string) string {
	if idx := strings.IndexByte(line, '#'); idx >= 0 {
		return line[:idx]
	}
	return line
}

func equalColumns(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func backup(filePath string, data []byte, archiveDir string) error {
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("create pg_hba.conf archive directory: %w", err)
	}
	name := time.Now().UTC().Format("2006_01_02_15_04_05") + "_" + filepath.Base(filePath)
	if err := os.WriteFile(filepath.Join(archiveDir, name), data, 0o600); err != nil {
		return fmt.Errorf("backup pg_hba.conf: %w", err)
	}
	return nil
}

func appendLine(filePath string, data []byte, line string) error {
	var b bytes.Buffer
	b.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString(line)
	b.WriteByte('\n')
	if err := os.WriteFile(filePath, b.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write pg_hba.conf %q: %w", filePath, err)
	}
	return nil
}
