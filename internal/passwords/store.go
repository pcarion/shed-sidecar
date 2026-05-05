package passwords

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	_ "modernc.org/sqlite"
)

const (
	lowercaseAlphabet = "abcdefghijklmnopqrstuvwxyz"
	uppercaseAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitAlphabet     = "0123456789"
	symbolAlphabet    = "!@#%^&*_-+=[]{}:,.?"
	hexLowerAlphabet  = "0123456789abcdef"
	hexUpperAlphabet  = "0123456789ABCDEF"
)

type Store struct {
	db *sql.DB
}

type Record struct {
	Value string
	IsNew bool
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	store := &Store{db: db}
	if err := store.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Get(ctx context.Context, service, name string, length int32, passwordType sidecarv1.PasswordType) (Record, error) {
	if strings.TrimSpace(service) == "" {
		return Record{}, errors.New("service name is required")
	}
	if strings.TrimSpace(name) == "" {
		return Record{}, errors.New("password name is required")
	}
	normalizedLength, err := normalizeLength(length, passwordType)
	if err != nil {
		return Record{}, err
	}

	if value, ok, err := s.lookup(ctx, service, name, normalizedLength, passwordType); err != nil {
		return Record{}, err
	} else if ok {
		return Record{Value: value}, nil
	}

	value, err := Generate(normalizedLength, passwordType)
	if err != nil {
		return Record{}, err
	}

	result, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO passwords (service, name, value, generationDate, length, "type")
VALUES (?, ?, ?, ?, ?, ?)`,
		service,
		name,
		value,
		time.Now().UTC().Format(time.RFC3339),
		normalizedLength,
		passwordType.String(),
	)
	if err != nil {
		return Record{}, fmt.Errorf("insert password: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Record{}, fmt.Errorf("read inserted password count: %w", err)
	}
	if rows == 0 {
		value, ok, err := s.lookup(ctx, service, name, normalizedLength, passwordType)
		if err != nil {
			return Record{}, err
		}
		if ok {
			return Record{Value: value}, nil
		}
		return Record{}, errors.New("password insert conflicted but existing password was not found")
	}
	return Record{Value: value, IsNew: true}, nil
}

func (s *Store) init(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS passwords (
	service TEXT NOT NULL,
	name TEXT NOT NULL,
	value TEXT NOT NULL,
	generationDate TEXT NOT NULL,
	length INTEGER NOT NULL,
	"type" TEXT NOT NULL,
	UNIQUE (service, name, length, "type")
)`); err != nil {
		return fmt.Errorf("create passwords table: %w", err)
	}
	return nil
}

func (s *Store) lookup(ctx context.Context, service, name string, length int32, passwordType sidecarv1.PasswordType) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
SELECT value
FROM passwords
WHERE service = ? AND name = ? AND length = ? AND "type" = ?`,
		service,
		name,
		length,
		passwordType.String(),
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookup password: %w", err)
	}
	return value, true, nil
}

func Generate(length int32, passwordType sidecarv1.PasswordType) (string, error) {
	normalizedLength, err := normalizeLength(length, passwordType)
	if err != nil {
		return "", err
	}
	switch passwordType {
	case sidecarv1.PasswordType_PASSWORD_TYPE_LOWERCASE:
		return randomString(normalizedLength, lowercaseAlphabet)
	case sidecarv1.PasswordType_PASSWORD_TYPE_UPPERCASE:
		return randomStringWithRequiredSets(normalizedLength, []string{lowercaseAlphabet, uppercaseAlphabet}, lowercaseAlphabet+uppercaseAlphabet)
	case sidecarv1.PasswordType_PASSWORD_TYPE_DIGIT:
		return randomString(normalizedLength, digitAlphabet)
	case sidecarv1.PasswordType_PASSWORD_TYPE_SYMBOL:
		return randomStringWithRequiredSets(normalizedLength, []string{lowercaseAlphabet, uppercaseAlphabet, symbolAlphabet}, lowercaseAlphabet+uppercaseAlphabet+symbolAlphabet)
	case sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER:
		return randomString(normalizedLength, hexLowerAlphabet)
	case sidecarv1.PasswordType_PASSWORD_TYPE_HEX_UPPER:
		return randomString(normalizedLength, hexUpperAlphabet)
	case sidecarv1.PasswordType_PASSWORD_TYPE_UUID_V7:
		return uuidV7()
	default:
		return "", fmt.Errorf("unsupported password type: %s", passwordType.String())
	}
}

func normalizeLength(length int32, passwordType sidecarv1.PasswordType) (int32, error) {
	if passwordType == sidecarv1.PasswordType_PASSWORD_TYPE_UUID_V7 {
		if length == 0 {
			return 36, nil
		}
		if length != 36 {
			return 0, errors.New("uuid v7 password length must be 36")
		}
		return length, nil
	}
	if length <= 0 {
		return 0, errors.New("password length must be positive")
	}
	return length, nil
}

func randomString(length int32, alphabet string) (string, error) {
	if length <= 0 {
		return "", errors.New("password length must be positive")
	}
	var b strings.Builder
	b.Grow(int(length))
	max := big.NewInt(int64(len(alphabet)))
	for i := int32(0); i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate random value: %w", err)
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}

func randomStringWithRequiredSets(length int32, requiredSets []string, alphabet string) (string, error) {
	if length < int32(len(requiredSets)) {
		return "", fmt.Errorf("password length must be at least %d for requested type", len(requiredSets))
	}
	out := make([]byte, length)
	for i, set := range requiredSets {
		ch, err := randomByte(set)
		if err != nil {
			return "", err
		}
		out[i] = ch
	}
	for i := len(requiredSets); i < int(length); i++ {
		ch, err := randomByte(alphabet)
		if err != nil {
			return "", err
		}
		out[i] = ch
	}
	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

func randomByte(alphabet string) (byte, error) {
	max := big.NewInt(int64(len(alphabet)))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, fmt.Errorf("generate random value: %w", err)
	}
	return alphabet[n.Int64()], nil
}

func shuffle(out []byte) error {
	for i := len(out) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("shuffle password: %w", err)
		}
		j := int(n.Int64())
		out[i], out[j] = out[j], out[i]
	}
	return nil
}

func uuidV7() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid random bytes: %w", err)
	}
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	), nil
}
