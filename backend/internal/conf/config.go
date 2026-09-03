package conf

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"codeup.aliyun.com/qimao/leo/leo/config"
	"codeup.aliyun.com/qimao/leo/leo/config/decoder"
	"codeup.aliyun.com/qimao/leo/leo/config/resource/file"
	"codeup.aliyun.com/qimao/leo/leo/config/resource/nacosv2"
)

// InitConfig 初始化配置
func InitConfig(serverName string) error {
	// 默认的mapstructure会导致kafka的配置无法读取
	config.DefaultMapStructureConfig.TagName = "json"
	// 文件包含nacos.yaml走nacos
	if _, err := os.Stat("./configs/nacos.yaml"); err == nil {
		// 读取远程配置，覆盖本地配置
		if _, err := newStrategyNacos(serverName, "nacos"); err != nil {
			return err
		}
		log.Println("nacos config file success")
		return nil
	}
	if _, err := os.Stat("./configs/config.yaml"); err == nil {
		// 文件夹包含config.yaml走config
		if _, err := newStrategyLocal("config"); err != nil {
			return err
		}
		log.Println("config init success")
		return nil
	}

	return errors.New("./config无nacos.yaml或者config.yaml")
}

func newStrategyNacos(serverName string, nacosFileName string) (config.Config, error) {
	// 先读取本地文件，读取nacos配置
	nacosLocalConfig, err := newStrategyLocal(nacosFileName)
	if err != nil {
		return nil, err
	}

	type nacosConfig struct {
		Endpoint  string `json:"endpoint"`
		Port      uint64 `json:"port"`
		Namespace string `json:"namespace"`
		Group     string `json:"group"`
		IpAddr    string `json:"ip_addr"`
	}

	nacosC := nacosConfig{}
	err = nacosLocalConfig.Get(serverName).Scan(&nacosC)
	if err != nil {
		return nil, err
	}
	factory := func() (config_client.IConfigClient, error) {
		sc := []constant.ServerConfig{
			*constant.NewServerConfig(nacosC.IpAddr, nacosC.Port, constant.WithContextPath("/nacos")),
		}
		cc := *constant.NewClientConfig(
			constant.WithNamespaceId(nacosC.Namespace),
			constant.WithTimeoutMs(5000),
			constant.WithNotLoadCacheAtStart(true),
			constant.WithLogDir("/tmp/nacos/log"),
			constant.WithCacheDir("/tmp/nacos/cache"),
			constant.WithLogLevel("debug"),
		)
		return clients.NewConfigClient(
			vo.NacosClientParam{
				ClientConfig:  &cc,
				ServerConfigs: sc,
			},
		)
	}
	yamlResource, err := nacosv2.NewResource("config.yaml", nacosC.Group, factory, nacosv2.Extension("yaml"))
	if err != nil {
		return nil, err
	}
	configure, err := config.NewConfigure(
		context.Background(),
		config.Decoders(decoder.YAML{}),
		config.Resources(yamlResource),
	)
	if err != nil {
		return nil, err
	}

	return configure, nil
}

// 根据mod加载配置
func newStrategyLocal(fileName string) (config.Config, error) {
	c, err := config.NewConfigure(
		context.Background(),
		config.Decoders(decoder.YAML{}),
		config.Resources(file.NewResource(fmt.Sprintf("./configs/%s.yaml", fileName))),
	)
	return c, err
}
