package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/spf13/cobra"
)

var bannerCmd = &cobra.Command{
	Use:   "banner [flame|classic|plain]",
	Short: "Switch banner style",
	Long:  "Cycle or set the banner style: flame (gradient), classic (green), plain (gray).",
	Args:  cobra.MaximumNArgs(1),
	ValidArgs: []string{"flame", "classic", "plain"},
	RunE:  runBanner,
}

func init() {
	rootCmd.AddCommand(bannerCmd)
}

func runBanner(cmd *cobra.Command, args []string) error {
	var next string
	if len(args) > 0 {
		next = args[0]
	} else {
		next = cycleBannerStyle(cfg.BannerStyle)
	}

	cfg.BannerStyle = next

	path := cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s Banner style set to %s\n",
		greenStyle.Render("✓"),
		greenStyle.Render(next))
	fmt.Fprintln(os.Stderr)
	return nil
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
