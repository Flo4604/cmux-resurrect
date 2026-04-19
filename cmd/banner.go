package cmd

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/spf13/cobra"
)

var bannerCmd = &cobra.Command{
	Use:   "banner",
	Short: "Manage banner style",
	Long:  "Manage the startup banner style: set, get, or list available styles.",
}

var bannerSetCmd = &cobra.Command{
	Use:       "set <flame|classic|plain>",
	Short:     "Set banner style",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"flame", "classic", "plain"},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.BannerStyle = args[0]
		path := cfgFile
		if path == "" {
			path = config.DefaultConfigPath()
		}
		if err := config.Save(path, cfg); err != nil {
			return err
		}
		fmt.Print(banner())
		return nil
	},
}

var bannerGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current banner style",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		style := cfg.BannerStyle
		if style == "" {
			style = "flame"
		}
		fmt.Printf("  Current banner style: %s\n", greenStyle.Render(style))
		return nil
	},
}

var bannerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available banner styles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("  Available banner styles:")
		fmt.Printf("    %s  gradient (ember → gold → green)\n", greenStyle.Render("flame  "))
		fmt.Printf("    %s  solid green\n", greenStyle.Render("classic"))
		fmt.Printf("    %s  monochrome gray\n", greenStyle.Render("plain  "))
		return nil
	},
}

func init() {
	bannerCmd.AddCommand(bannerSetCmd)
	bannerCmd.AddCommand(bannerGetCmd)
	bannerCmd.AddCommand(bannerListCmd)
	rootCmd.AddCommand(bannerCmd)
}

// cycleBannerStyle returns the next style in the cycle: flame → classic → plain → flame.
func cycleBannerStyle(current string) string {
	switch current {
	case "classic":
		return "plain"
	case "plain":
		return "flame"
	default:
		return "classic"
	}
}
