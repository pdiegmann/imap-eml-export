package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pdiegmann/imap-eml-export/internal/config"
	"github.com/pdiegmann/imap-eml-export/internal/export"
	"github.com/pdiegmann/imap-eml-export/internal/imapclient"
	"github.com/pdiegmann/imap-eml-export/internal/importer"
	"github.com/pdiegmann/imap-eml-export/internal/tui"
	"github.com/pdiegmann/imap-eml-export/internal/updater"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "imap-eml-export",
	Short: "Export IMAP mailboxes to EML files",
	Long:  "A tool to export all emails from an IMAP server to local EML files, mirroring the folder hierarchy.",
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export emails from IMAP server",
	RunE:  runExport,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import EML files into an IMAP server",
	Long:  "Reads exported EML files from a local directory and uploads them to a target IMAP server, preserving the original folder hierarchy.",
	RunE:  runImport,
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update imap-eml-export to the latest version",
	RunE:  runUpdate,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("imap-eml-export %s\n", version)
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "config file path")
	rootCmd.PersistentFlags().String("log-file", "", "log file path")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().Bool("debug", false, "debug output")

	exportCmd.Flags().String("host", "", "IMAP host")
	exportCmd.Flags().Int("port", 0, "IMAP port")
	exportCmd.Flags().StringP("username", "u", "", "IMAP username")
	exportCmd.Flags().StringP("password", "p", "", "IMAP password")
	exportCmd.Flags().StringP("output", "o", "", "output directory")
	exportCmd.Flags().Bool("tls", true, "use TLS")
	exportCmd.Flags().Bool("starttls", false, "use STARTTLS")
	exportCmd.Flags().BoolP("yes", "y", false, "skip confirmations")

	viper.BindPFlag("host", exportCmd.Flags().Lookup("host"))         //nolint:errcheck
	viper.BindPFlag("port", exportCmd.Flags().Lookup("port"))         //nolint:errcheck
	viper.BindPFlag("username", exportCmd.Flags().Lookup("username")) //nolint:errcheck
	viper.BindPFlag("password", exportCmd.Flags().Lookup("password")) //nolint:errcheck
	viper.BindPFlag("output_dir", exportCmd.Flags().Lookup("output")) //nolint:errcheck
	viper.BindPFlag("tls", exportCmd.Flags().Lookup("tls"))           //nolint:errcheck
	viper.BindPFlag("starttls", exportCmd.Flags().Lookup("starttls")) //nolint:errcheck

	importCmd.Flags().String("host", "", "target IMAP host")
	importCmd.Flags().Int("port", 0, "target IMAP port")
	importCmd.Flags().StringP("username", "u", "", "target IMAP username")
	importCmd.Flags().StringP("password", "p", "", "target IMAP password")
	importCmd.Flags().StringP("input", "i", "", "input directory containing exported EML files")
	importCmd.Flags().Bool("tls", true, "use TLS for target connection")
	importCmd.Flags().Bool("starttls", false, "use STARTTLS for target connection")

	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	cfgFile, _ := cmd.Root().PersistentFlags().GetString("config")
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if host, _ := cmd.Flags().GetString("host"); host != "" {
		cfg.Host = host
	}
	if port, _ := cmd.Flags().GetInt("port"); port != 0 {
		cfg.Port = port
	}
	if username, _ := cmd.Flags().GetString("username"); username != "" {
		cfg.Username = username
	}
	if password, _ := cmd.Flags().GetString("password"); password != "" {
		cfg.Password = password
	}
	if output, _ := cmd.Flags().GetString("output"); output != "" {
		cfg.OutputDir = output
	}

	if cfg.Host == "" {
		cfg, err = tui.RunWizard()
		if err != nil {
			return fmt.Errorf("wizard: %w", err)
		}
		if cfgPath, err := config.DefaultConfigPath(); err == nil {
			_ = cfg.Save(cfgPath)
		}
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	updates := make(chan export.ProgressUpdate, 100)
	progressDone := make(chan error, 1)

	go func() {
		progressDone <- tui.RunProgress(ctx, updates)
	}()

	client := imapclient.New(cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.TLS, cfg.StartTLS)

	if err := client.Connect(); err != nil {
		close(updates)
		return fmt.Errorf("connecting: %w", err)
	}
	defer client.Close()

	if err := client.Authenticate(); err != nil {
		close(updates)
		return fmt.Errorf("authenticating: %w", err)
	}

	exporter := export.New(cfg.OutputDir, func(u export.ProgressUpdate) {
		select {
		case updates <- u:
		default:
		}
	})

	exportErr := exporter.Export(ctx, client)
	close(updates)
	<-progressDone

	return exportErr
}

func runImport(cmd *cobra.Command, args []string) error {
	cfgFile, _ := cmd.Root().PersistentFlags().GetString("config")
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// CLI flag overrides (target IMAP credentials).
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		cfg.Host = host
	}
	if port, _ := cmd.Flags().GetInt("port"); port != 0 {
		cfg.Port = port
	}
	if username, _ := cmd.Flags().GetString("username"); username != "" {
		cfg.Username = username
	}
	if password, _ := cmd.Flags().GetString("password"); password != "" {
		cfg.Password = password
	}

	// Determine the input directory (separate from cfg.OutputDir).
	inputDir, _ := cmd.Flags().GetString("input")
	if inputDir == "" {
		inputDir = cfg.OutputDir // fall back to the config's output_dir as a convenience
	}

	// Validate required fields for import (no outputDir required).
	if cfg.Host == "" {
		return fmt.Errorf("target IMAP host is required (use --host or set host in config)")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Port)
	}
	if cfg.Username == "" {
		return fmt.Errorf("target IMAP username is required (use --username or set username in config)")
	}
	if cfg.Password == "" {
		return fmt.Errorf("target IMAP password is required (use --password or set password in config)")
	}
	if inputDir == "" {
		return fmt.Errorf("input directory is required (use --input or set output_dir in config)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	updates := make(chan export.ProgressUpdate, 100)
	progressDone := make(chan error, 1)

	go func() {
		progressDone <- tui.RunImportProgress(ctx, updates)
	}()

	tlsFlag, _ := cmd.Flags().GetBool("tls")
	startTLSFlag, _ := cmd.Flags().GetBool("starttls")
	client := imapclient.New(cfg.Host, cfg.Port, cfg.Username, cfg.Password, tlsFlag, startTLSFlag)

	if err := client.Connect(); err != nil {
		close(updates)
		return fmt.Errorf("connecting to target: %w", err)
	}
	defer client.Close()

	if err := client.Authenticate(); err != nil {
		close(updates)
		return fmt.Errorf("authenticating with target: %w", err)
	}

	imp := importer.New(inputDir, func(u export.ProgressUpdate) {
		select {
		case updates <- u:
		default:
		}
	})

	importErr := imp.Import(ctx, client)
	close(updates)
	<-progressDone

	return importErr
}

func runUpdate(cmd *cobra.Command, args []string) error {
	latest, hasUpdate, err := updater.CheckUpdate(version)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	if !hasUpdate {
		fmt.Printf("Already up to date (version %s)\n", version)
		return nil
	}
	fmt.Printf("Update available: %s → %s\n", version, latest)
	if err := updater.DoUpdate(version); err != nil {
		return fmt.Errorf("updating: %w", err)
	}
	fmt.Println("Update complete! Please restart the application.")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
