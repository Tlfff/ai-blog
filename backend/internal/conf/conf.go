package conf

import (
	"codeup.aliyun.com/qimao/leo/leo/config"
	"codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/sentry"
)

func NewConfig() (*Config, error) {
	var data Config
	if err := config.Get("").Scan(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

func GetConfigData(cf *Config) *Data {
	return cf.Data
}

func GetConfigServer(cf *Config) *Server {
	return cf.Server
}

func (conf *Sentry) ToSentryConfig() *sentry.Sentry {
	return &sentry.Sentry{
		Dsn:         conf.GetDsn(),
		Environment: conf.GetEnvironment(),
	}
}
