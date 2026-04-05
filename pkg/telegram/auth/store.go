package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// ErrInvalidOTP is returned when an OTP does not match any pending request.
var ErrInvalidOTP = errors.New("invalid OTP")

// Data is the on-disk representation of telegram.json.
type Data struct {
	AllowedUsers []int64           `json:"allowed_users"`
	PendingOTPs  map[string]string `json:"pending_otps"` // userID (string) → OTP
}

// Store provides thread-safe access to Telegram auth state.
type Store struct {
	mu   sync.RWMutex
	path string
	data Data
}

// New loads an existing telegram.json or initializes empty state.
func New(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: Data{
			PendingOTPs: make(map[string]string),
		},
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, s.persist()
		}
		return nil, fmt.Errorf("read telegram auth: %w", err)
	}

	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("parse telegram auth: %w", err)
	}
	if s.data.PendingOTPs == nil {
		s.data.PendingOTPs = make(map[string]string)
	}

	return s, nil
}

// IsAllowed returns true if the user ID is in the allowed list.
func (s *Store) IsAllowed(userID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.data.AllowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}

// GetOrCreateOTP returns the existing OTP for the user or generates a new one.
func (s *Store) GetOrCreateOTP(userID int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strconv.FormatInt(userID, 10)
	if otp, ok := s.data.PendingOTPs[key]; ok {
		return otp
	}

	otp := generateOTP()
	s.data.PendingOTPs[key] = otp
	if err := s.persist(); err != nil {
		slog.Error("persist telegram auth", "error", err)
	}
	return otp
}

// ApproveByOTP finds the user with the given OTP, moves them to the allowed
// list, and removes the pending OTP. Returns the approved user ID.
func (s *Store) ApproveByOTP(otp string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, v := range s.data.PendingOTPs {
		if v == otp {
			userID, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parse user id: %w", err)
			}
			s.data.AllowedUsers = append(s.data.AllowedUsers, userID)
			delete(s.data.PendingOTPs, key)
			if err := s.persist(); err != nil {
				return 0, err
			}
			return userID, nil
		}
	}

	return 0, ErrInvalidOTP
}

// persist writes the current state to disk atomically.
// Caller must hold s.mu.
func (s *Store) persist() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal telegram auth: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "telegram-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}

	if err := os.Rename(tmp.Name(), s.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// generateOTP returns a 6-character alphanumeric code.
// Ambiguous characters (O, 0, I, 1) are excluded.
func generateOTP() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
