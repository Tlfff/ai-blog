package clients

import (
	log2 "codeup.aliyun.com/qimao/blog/ai-blog/backend/third_party/pkg"
	"codeup.aliyun.com/qimao/leo/driver/mysqlxorm"
	"codeup.aliyun.com/qimao/leo/leo/config"
	"codeup.aliyun.com/qimao/leo/leo/log"
	_ "github.com/go-sql-driver/mysql"
)

// NewMysqlClient 创建博客业务数据库客户端。
func NewMysqlClient() (MysqlClient, func(), error) {
	var simpleConfig mysqlxorm.SimpleConfig
	if err := config.Get("data.mysql.default_db").Scan(&simpleConfig); err != nil {
		return nil, nil, err
	}
	clientDB, fn, err := mysqlxorm.NewSimpleClient(simpleConfig)
	if err == nil {
		logger := log.L()
		clientDB.SetLogger(log2.NewORMLogger(logger))
	}
	return clientDB, fn, err
}

// 操作日志数据库
func NewLogMysqlClient() (MysqlLogClient, func(), error) {
	var simpleConfig mysqlxorm.SimpleConfig
	if err := config.Get("data.mysql.log_db").Scan(&simpleConfig); err != nil {
		return nil, nil, err
	}
	clientDB, fn, err := mysqlxorm.NewSimpleClient(simpleConfig)
	if err == nil {
		logger := log.L()
		clientDB.SetLogger(log2.NewORMLogger(logger))
	}
	return clientDB, fn, err
}
