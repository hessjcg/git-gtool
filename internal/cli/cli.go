package cli

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hessjcg/git-gtool/internal/model"
	"github.com/hessjcg/git-gtool/internal/renovatepr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	org         string
	base        string
	userLicense string
	repos       = make([]string, 0, 10)

	rootCmd = &cobra.Command{
		Use:   "git-gtool",
		Short: "Runs tools on the github API",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("The github issue notifier")
		},
	}

	renovatePrs = &cobra.Command{
		Use:   "merge-renovate-prs",
		Short: "Merges open prs from RenovateBot. This will run for several minutes until all PRs are merged",
		Run: func(cmd *cobra.Command, args []string) {
			var cwd, _ = os.Getwd()
			ctx := context.Background()
			client, err := model.NewClient(ctx, cwd)
			if err != nil {
				log.Fatalf("Can't get client: %v", err)
			}
			for _, repo := range repos {
				err = renovatepr.MergePrs(ctx, client, org, repo, base)
				if err != nil {
					log.Fatalf("Can't merge renovate PRs for %v/%v: %v", org, repo, err)
				}
			}
		},
	}
)

func init() {
	log.SetFlags(0)
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	rootCmd.PersistentFlags().StringVar(&org, "org", "GoogleCloudPlatform", "Github Organization")
	rootCmd.PersistentFlags().StringVar(&base, "base", "", "Base branch for PRs")
	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	rootCmd.PersistentFlags().StringArrayVar(&repos, "repo", []string{}, "List of repos to analyze.")
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
