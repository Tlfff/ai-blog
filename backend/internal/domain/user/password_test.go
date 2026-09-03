package user

import (
	"strings"
	"testing"
)

// TestPBKDF2PasswordHasherUsesExpectedFormat 验证密码摘要格式、迭代次数和随机 Salt。
func TestPBKDF2PasswordHasherUsesExpectedFormat(t *testing.T) {
	t.Parallel()

	hasher := NewPBKDF2PasswordHasher()
	first, err := hasher.Hash("secret1")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	second, err := hasher.Hash("secret1")
	if err != nil {
		t.Fatalf("Hash() second error = %v", err)
	}

	parts := strings.Split(first, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" || parts[1] != "100000" {
		t.Fatalf("Hash() format = %q", first)
	}
	if first == second {
		t.Fatal("Hash() should use a random salt")
	}
}
