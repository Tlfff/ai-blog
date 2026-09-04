package repo

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
)

// TestNewRepositoryRequiresTransactions 验证文章仓储禁止非事务客户端降级。
func TestNewRepositoryRequiresTransactions(t *testing.T) {
	// 1. 仅具备普通 XORM 能力的客户端必须在启动阶段失败
	var client clients.MysqlClient = new(nonTransactionalClient)
	defer func() {
		if recover() == nil {
			t.Fatal("NewRepository() did not panic for non-transactional client")
		}
	}()
	NewRepository(client, nil)
}

// nonTransactionalClient 用于表达不具备 Transaction 的测试客户端。
type nonTransactionalClient struct {
	clients.MysqlClient // MysqlClient 提供普通数据库接口，但运行时值不实现事务扩展。
}
