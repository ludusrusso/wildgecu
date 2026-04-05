package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("creates file when missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil store")
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file to be created: %v", err)
		}
	})

	t.Run("loads existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		data := Data{
			AllowedUsers: []int64{111, 222},
			PendingOTPs:  map[string]string{"333": "ABC123"},
		}
		raw, _ := json.Marshal(data)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := New(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !s.IsAllowed(111) {
			t.Error("expected user 111 to be allowed")
		}
		if !s.IsAllowed(222) {
			t.Error("expected user 222 to be allowed")
		}
		if otp := s.GetOrCreateOTP(333); otp != "ABC123" {
			t.Errorf("expected existing OTP ABC123, got %q", otp)
		}
	})

	t.Run("handles nil pending_otps in file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		if err := os.WriteFile(path, []byte(`{"allowed_users":[1]}`), 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := New(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should not panic when generating OTP with nil map
		otp := s.GetOrCreateOTP(999)
		if otp == "" {
			t.Error("expected non-empty OTP")
		}
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		if err := os.WriteFile(path, []byte(`{invalid`), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := New(path)
		if err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}

func TestIsAllowed(t *testing.T) {
	s := &Store{
		data: Data{
			AllowedUsers: []int64{100, 200},
			PendingOTPs:  make(map[string]string),
		},
	}

	t.Run("returns true for allowed user", func(t *testing.T) {
		if !s.IsAllowed(100) {
			t.Error("expected user 100 to be allowed")
		}
	})

	t.Run("returns false for unknown user", func(t *testing.T) {
		if s.IsAllowed(999) {
			t.Error("expected user 999 to not be allowed")
		}
	})

	t.Run("returns false on empty list", func(t *testing.T) {
		empty := &Store{data: Data{PendingOTPs: make(map[string]string)}}
		if empty.IsAllowed(1) {
			t.Error("expected false on empty allowed list")
		}
	})
}

func TestGetOrCreateOTP(t *testing.T) {
	t.Run("generates OTP for new user", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp := s.GetOrCreateOTP(42)
		if len(otp) != 6 {
			t.Errorf("expected 6-char OTP, got %q", otp)
		}
	})

	t.Run("returns same OTP on repeated calls", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp1 := s.GetOrCreateOTP(42)
		otp2 := s.GetOrCreateOTP(42)
		if otp1 != otp2 {
			t.Errorf("expected same OTP, got %q and %q", otp1, otp2)
		}
	})

	t.Run("different users get different OTPs", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp1 := s.GetOrCreateOTP(1)
		otp2 := s.GetOrCreateOTP(2)
		if otp1 == otp2 {
			// Technically possible but astronomically unlikely with 30^6 space
			t.Errorf("expected different OTPs for different users, both got %q", otp1)
		}
	})

	t.Run("persists to disk", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp := s.GetOrCreateOTP(42)

		// Reload from disk
		s2, err := New(path)
		if err != nil {
			t.Fatal(err)
		}
		otp2 := s2.GetOrCreateOTP(42)
		if otp != otp2 {
			t.Errorf("OTP not persisted: got %q after reload, expected %q", otp2, otp)
		}
	})
}

func TestApproveByOTP(t *testing.T) {
	t.Run("approves valid OTP", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp := s.GetOrCreateOTP(42)
		userID, err := s.ApproveByOTP(otp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if userID != 42 {
			t.Errorf("expected user ID 42, got %d", userID)
		}
		if !s.IsAllowed(42) {
			t.Error("expected user 42 to be allowed after approval")
		}
	})

	t.Run("rejects invalid OTP", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		_, err = s.ApproveByOTP("NOPE00")
		if err != ErrInvalidOTP {
			t.Errorf("expected ErrInvalidOTP, got %v", err)
		}
	})

	t.Run("OTP consumed after approval", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp := s.GetOrCreateOTP(42)
		if _, err := s.ApproveByOTP(otp); err != nil {
			t.Fatal(err)
		}

		// Same OTP should now fail
		_, err = s.ApproveByOTP(otp)
		if err != ErrInvalidOTP {
			t.Errorf("expected ErrInvalidOTP on reuse, got %v", err)
		}
	})

	t.Run("persists approval to disk", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telegram.json")
		s, err := New(path)
		if err != nil {
			t.Fatal(err)
		}

		otp := s.GetOrCreateOTP(42)
		if _, err := s.ApproveByOTP(otp); err != nil {
			t.Fatal(err)
		}

		// Reload from disk
		s2, err := New(path)
		if err != nil {
			t.Fatal(err)
		}
		if !s2.IsAllowed(42) {
			t.Error("expected user 42 to be allowed after reload")
		}
		// OTP should be gone
		_, err = s2.ApproveByOTP(otp)
		if err != ErrInvalidOTP {
			t.Errorf("expected OTP to be consumed after reload, got %v", err)
		}
	})
}

func TestGenerateOTP(t *testing.T) {
	t.Run("returns 6 characters", func(t *testing.T) {
		otp := generateOTP()
		if len(otp) != 6 {
			t.Errorf("expected 6 chars, got %d: %q", len(otp), otp)
		}
	})

	t.Run("uses only allowed charset", func(t *testing.T) {
		allowed := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
		for i := 0; i < 100; i++ {
			otp := generateOTP()
			for _, c := range otp {
				found := false
				for _, a := range allowed {
					if c == a {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("OTP %q contains disallowed character %c", otp, c)
				}
			}
		}
	})
}
