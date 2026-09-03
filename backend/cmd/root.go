package cmd

import (
	"fmt"
	"os"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/cmd/backend"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/cmd/consumer"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/cmd/job"
	"github.com/spf13/cobra"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "blog",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(server.HttpCmd)
	rootCmd.AddCommand(server.GrpcCmd)
	rootCmd.AddCommand(job.JobCmd)
	rootCmd.AddCommand(consumer.ConsumerCmd)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
