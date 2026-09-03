package repo

import (
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/go-sql-driver/mysql"
)

// TestMapDuplicateError 验证数据库唯一索引冲突映射为稳定领域错误。
func TestMapDuplicateError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string // name 是测试场景名称。
		message string // message 是 MySQL 唯一索引错误信息。
		want    error  // want 是预期领域错误。
	}{
		{name: "昵称唯一索引", message: "Duplicate entry 'tester' for key 'users.uni_nickname'", want: user.ErrNicknameExists},
		{name: "手机号唯一索引", message: "Duplicate entry '13800138000' for key 'users.uni_phone'", want: user.ErrPhoneExists},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := mapDuplicateError(&mysql.MySQLError{Number: 1062, Message: tt.message})
			if !errors.Is(err, tt.want) {
				t.Fatalf("mapDuplicateError() = %v, want %v", err, tt.want)
			}
		})
	}
}
