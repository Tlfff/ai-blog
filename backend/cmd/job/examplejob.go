package job

import (
	"github.com/spf13/cobra"
)

var blogJobCmd = &cobra.Command{
	Use:   "blog",
	Short: "blog",
	Long:  `运行博客任务`,
	Run: func(cmd *cobra.Command, args []string) {
		job, cancel, err := NewBlogJob()
		if err != nil {
			panic(err)
		}
		defer cancel()
		job.Run()
	},
}

func init() {
	JobCmd.AddCommand(blogJobCmd)
}
