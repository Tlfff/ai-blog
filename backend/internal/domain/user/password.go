package user

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordIterations = 100000 // passwordIterations 是兼容格式使用的 PBKDF2 迭代次数。
	passwordSaltSize   = 16     // passwordSaltSize 是随机 Salt 的字节数。
	passwordKeySize    = 32     // passwordKeySize 是 SHA-256 派生密钥的字节数。
)

// PasswordHasher 定义用户密码摘要能力。
type PasswordHasher interface {
	// Hash 使用随机 Salt 生成不可逆密码摘要。
	Hash(password string) (string, error)
	// Compare 使用恒定时间比较明文密码和已保存摘要。
	Compare(encoded, password string) (bool, error)
}

// PBKDF2PasswordHasher 使用 PBKDF2-SHA256 生成兼容密码摘要。
type PBKDF2PasswordHasher struct{}

// NewPBKDF2PasswordHasher 创建 PBKDF2-SHA256 密码摘要器。
func NewPBKDF2PasswordHasher() PasswordHasher {
	return &PBKDF2PasswordHasher{}
}

// Hash 使用安全随机 Salt 生成 `pbkdf2$迭代次数$Salt$Hash` 格式摘要。
func (h *PBKDF2PasswordHasher) Hash(password string) (string, error) {
	// 1. 生成每次摘要独立的安全随机 Salt
	salt := make([]byte, passwordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成密码 Salt: %w", err)
	}

	// 2. 按兼容参数派生密钥并编码为稳定存储格式
	key := pbkdf2.Key([]byte(password), salt, passwordIterations, passwordKeySize, sha256.New)
	return fmt.Sprintf(
		"pbkdf2$%d$%s$%s",
		passwordIterations,
		hex.EncodeToString(salt),
		hex.EncodeToString(key),
	), nil
}

// Compare 校验 PBKDF2 密码摘要。
func (h *PBKDF2PasswordHasher) Compare(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return false, errors.New("密码摘要格式错误")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, errors.New("密码摘要迭代参数错误")
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false, errors.New("密码摘要 Salt 错误")
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return false, errors.New("密码摘要 Hash 错误")
	}
	got := pbkdf2.Key([]byte(password), salt, iterations, len(want), sha256.New)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
