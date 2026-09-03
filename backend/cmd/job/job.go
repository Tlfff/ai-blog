package job

import (
	"github.com/spf13/cobra"
)

// JobCmd 脚本入口
// 执行方式：go run main.go job blog。
var JobCmd = &cobra.Command{
	Use:   "job",
	Short: "leo-job",
}
