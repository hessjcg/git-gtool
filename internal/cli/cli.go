package cli

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hessjcg/git-gtool/internal/gitrepo"
	"github.com/hessjcg/git-gtool/internal/renovatepr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	userLicense string

	rootCmd = &cobra.Command{
		Use:   "git-gtool",
		Short: "Runs tools on local git repos to help with git and github admin.",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("The github issue notifier")
		},
	}

	renovatePrs = &cobra.Command{
		Use:   "merge-renovate-prs",
		Short: "Merges open prs from RenovateBot.",
		Long: "This will run for several minutes until all PRs are merged.\n" +
			"It iterates over open renovate PRs and attempts to merge them\n" +
			"one by one.",
		Run: func(cmd *cobra.Command, args []string) {
			var cwd, _ = os.Getwd()
			ctx := context.Background()
			repo, err := gitrepo.OpenGit(ctx, cwd)
			if err != nil {
				log.Fatalf("Unable to open github client: %v", err)
			}
			err = renovatepr.MergePRs(ctx, repo)
		},
	}
)

func init() {
	log.SetFlags(0)
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))

	rootCmd.AddCommand(renovatePrs)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cobra")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
