package keyvalue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

type Result struct {
	Value string
	Type  sidecarv1.KeyValueValueType
	Found bool
}

func Configure(req *sidecarv1.ConfigureKeyValueConfRequest, configDir string) (bool, error) {
	filePath := resolvePath(req.GetFilePath(), configDir)
	if filePath == "" {
		return false, fmt.Errorf("configuration file path is required")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("read configuration file %q: %w", filePath, err)
	}

	lines := splitLines(data)
	changed := false
	for _, entry := range req.GetEntries() {
		key := strings.TrimSpace(entry.GetKey())
		if key == "" {
			return false, fmt.Errorf("configuration key is required")
		}
		if findActive(lines, req.GetType(), key) >= 0 {
			continue
		}
		line := formatLine(req.GetType(), key, entry.GetValue(), entry.GetType())
		insertAt := findComment(lines, key)
		if insertAt >= 0 {
			lines = insertLine(lines, insertAt+1, line)
		} else {
			lines = append(lines, line)
		}
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := backup(filePath, data, filepath.Join(configDir, "archive")); err != nil {
		return false, err
	}
	if err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return false, fmt.Errorf("write configuration file %q: %w", filePath, err)
	}
	return true, nil
}

func Get(req *sidecarv1.ConfigureGetKeyValueRequest, configDir string) (Result, error) {
	filePath := resolvePath(req.GetFilePath(), configDir)
	if filePath == "" {
		return Result{}, fmt.Errorf("configuration file path is required")
	}
	key := strings.TrimSpace(req.GetKey())
	if key == "" {
		return Result{}, fmt.Errorf("configuration key is required")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Result{}, fmt.Errorf("read configuration file %q: %w", filePath, err)
	}
	for _, line := range splitLines(data) {
		if isComment(line) || strings.TrimSpace(line) == "" {
			continue
		}
		gotKey, value, ok := parseLine(req.GetType(), line)
		if ok && gotKey == key {
			valueType := sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER
			if strings.HasPrefix(strings.TrimSpace(value), `"`) {
				valueType = sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING
				if unquoted, err := strconv.Unquote(strings.TrimSpace(value)); err == nil {
					value = unquoted
				}
			}
			return Result{Value: strings.TrimSpace(value), Type: valueType, Found: true}, nil
		}
	}
	return Result{}, nil
}

func resolvePath(path, configDir string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(configDir, path)
}

func splitLines(data []byte) []string {
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func isComment(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "#")
}

func findActive(lines []string, confType sidecarv1.KeyValueConfType, key string) int {
	for i, line := range lines {
		if isComment(line) || strings.TrimSpace(line) == "" {
			continue
		}
		if gotKey, _, ok := parseLine(confType, line); ok && gotKey == key {
			return i
		}
	}
	return -1
}

func findComment(lines []string, key string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "#")))
		if len(fields) > 0 && fields[0] == key {
			return i
		}
	}
	return -1
}

func parseLine(confType sidecarv1.KeyValueConfType, line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	switch confType {
	case sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL:
		return parseDelimited(trimmed, "=")
	case sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_COLON:
		return parseDelimited(trimmed, ":")
	case sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_SPACE:
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return "", "", false
		}
		return fields[0], strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0])), true
	default:
		return "", "", false
	}
}

func parseDelimited(line, delimiter string) (string, string, bool) {
	parts := strings.SplitN(line, delimiter, 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(parts[1]), true
}

func formatLine(confType sidecarv1.KeyValueConfType, key, value string, valueType sidecarv1.KeyValueValueType) string {
	formatted := formatValue(value, valueType)
	switch confType {
	case sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL:
		return key + " = " + formatted
	case sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_COLON:
		return key + " : " + formatted
	default:
		return key + " " + formatted
	}
}

func formatValue(value string, valueType sidecarv1.KeyValueValueType) string {
	if valueType == sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER {
		return value
	}
	return strconv.Quote(value)
}

func insertLine(lines []string, index int, line string) []string {
	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = line
	return lines
}

func backup(filePath string, data []byte, archiveDir string) error {
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("create configuration archive directory: %w", err)
	}
	name := time.Now().UTC().Format("2006_01_02_15_04_05") + "_" + filepath.Base(filePath)
	if err := os.WriteFile(filepath.Join(archiveDir, name), data, 0o600); err != nil {
		return fmt.Errorf("backup configuration file: %w", err)
	}
	return nil
}
