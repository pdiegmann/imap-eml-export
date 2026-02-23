package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pdiegmann/imap-eml-export/internal/config"
	"github.com/pdiegmann/imap-eml-export/internal/export"
	"github.com/pdiegmann/imap-eml-export/internal/google"
	"github.com/pdiegmann/imap-eml-export/internal/imapclient"
	"github.com/pdiegmann/imap-eml-export/internal/importer"
	"github.com/pdiegmann/imap-eml-export/internal/tui"
	"github.com/pdiegmann/imap-eml-export/internal/updater"
	"github.com/spf13/cobra"
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

	// export command flags – these override the [export] config section.
	exportCmd.Flags().String("export-host", "", "IMAP host for export")
	exportCmd.Flags().Int("export-port", 0, "IMAP port for export")
	exportCmd.Flags().StringP("export-username", "u", "", "IMAP username for export")
	exportCmd.Flags().StringP("export-password", "p", "", "IMAP password for export")
	exportCmd.Flags().StringP("output", "o", "", "output directory")
	exportCmd.Flags().Bool("export-tls", true, "use TLS for export connection")
	exportCmd.Flags().Bool("export-starttls", false, "use STARTTLS for export connection")
	exportCmd.Flags().Bool("google", false, "use Google/Gmail OAuth2 for export (sets host/port/TLS automatically)")
	exportCmd.Flags().BoolP("yes", "y", false, "skip confirmations")

	// import command flags – these override the [import] config section.
	importCmd.Flags().String("import-host", "", "target IMAP host for import")
	importCmd.Flags().Int("import-port", 0, "target IMAP port for import")
	importCmd.Flags().StringP("import-username", "u", "", "target IMAP username for import")
	importCmd.Flags().StringP("import-password", "p", "", "target IMAP password for import")
	importCmd.Flags().StringP("input", "i", "", "input directory containing exported EML files")
	importCmd.Flags().Bool("import-tls", true, "use TLS for import connection")
	importCmd.Flags().Bool("import-starttls", false, "use STARTTLS for import connection")
	importCmd.Flags().Bool("google", false, "use Google/Gmail OAuth2 for import (sets host/port/TLS automatically)")

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

	// Apply CLI flag overrides for the export section.
	if v, _ := cmd.Flags().GetString("export-host"); v != "" {
		cfg.Export.Host = v
	}
	if v, _ := cmd.Flags().GetInt("export-port"); v != 0 {
		cfg.Export.Port = v
	}
	if v, _ := cmd.Flags().GetString("export-username"); v != "" {
		cfg.Export.Username = v
	}
	if v, _ := cmd.Flags().GetString("export-password"); v != "" {
		cfg.Export.Password = v
	}
	if v, _ := cmd.Flags().GetString("output"); v != "" {
		cfg.Export.OutputDir = v
	}
	if googleFlag, _ := cmd.Flags().GetBool("google"); googleFlag {
		cfg.Export.Google = true
	}

	// Apply Google preset: auto-set host/port/TLS when Google mode is active.
	if cfg.Export.Google {
		if cfg.Export.Host == "" {
			cfg.Export.Host = google.GmailIMAPHost
		}
		// Always use the standard IMAPS port for Google.
		cfg.Export.Port = google.GmailIMAPPort
		cfg.Export.TLS = true
	}

	if cfg.Export.Host == "" {
		cfg, err = tui.RunWizard()
		if err != nil {
			return fmt.Errorf("wizard: %w", err)
		}
		if cfgPath, err := config.DefaultConfigPath(); err == nil {
			_ = cfg.Save(cfgPath)
		}
	}

	if err := cfg.ValidateExport(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	updates := make(chan export.ProgressUpdate, 100)
	progressDone := make(chan error, 1)

	go func() {
		progressDone <- tui.RunProgress(ctx, updates)
	}()

	client := imapclient.New(cfg.Export.Host, cfg.Export.Port, cfg.Export.Username, cfg.Export.Password, cfg.Export.TLS, cfg.Export.StartTLS)

	if err := client.Connect(); err != nil {
		close(updates)
		return fmt.Errorf("connecting: %w", err)
	}
	defer client.Close()

	if cfg.Export.Google {
		// OAuth2 OAUTHBEARER authentication for Gmail/GSuite.
		accessToken, newRefresh, err := google.GetAccessToken(
			ctx,
			cfg.Export.OAuth2.ClientID,
			cfg.Export.OAuth2.ClientSecret,
			cfg.Export.OAuth2.RefreshToken,
			"",
		)
		if err != nil {
			close(updates)
			return fmt.Errorf("obtaining Google access token: %w", err)
		}
		// Persist the refresh token if it changed.
		if newRefresh != "" && newRefresh != cfg.Export.OAuth2.RefreshToken {
			cfg.Export.OAuth2.RefreshToken = newRefresh
			if cfgPath, err := config.DefaultConfigPath(); err == nil {
				_ = cfg.Save(cfgPath)
			}
		}
		if err := client.AuthenticateOAuth2(accessToken); err != nil {
			close(updates)
			return fmt.Errorf("authenticating with Google: %w", err)
		}
	} else {
		if err := client.Authenticate(); err != nil {
			close(updates)
			return fmt.Errorf("authenticating: %w", err)
		}
	}

	exporter := export.New(cfg.Export.OutputDir, func(u export.ProgressUpdate) {
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

	// Apply CLI flag overrides for the import section.
	if v, _ := cmd.Flags().GetString("import-host"); v != "" {
		cfg.Import.Host = v
	}
	if v, _ := cmd.Flags().GetInt("import-port"); v != 0 {
		cfg.Import.Port = v
	}
	if v, _ := cmd.Flags().GetString("import-username"); v != "" {
		cfg.Import.Username = v
	}
	if v, _ := cmd.Flags().GetString("import-password"); v != "" {
		cfg.Import.Password = v
	}
	if googleFlag, _ := cmd.Flags().GetBool("google"); googleFlag {
		cfg.Import.Google = true
	}

	// Apply Google preset.
	if cfg.Import.Google {
		if cfg.Import.Host == "" {
			cfg.Import.Host = google.GmailIMAPHost
		}
		// Always use the standard IMAPS port for Google.
		cfg.Import.Port = google.GmailIMAPPort
		cfg.Import.TLS = true
	}

	// Determine the input directory.
	inputDir, _ := cmd.Flags().GetString("input")
	if inputDir == "" {
		inputDir = cfg.Import.InputDir
	}
	if inputDir == "" {
		// Fall back to the export output_dir as a convenience.
		inputDir = cfg.Export.OutputDir
	}

	if err := cfg.ValidateImport(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if inputDir == "" {
		return fmt.Errorf("input directory is required (use --input or set import.input_dir / export.output_dir in config)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	updates := make(chan export.ProgressUpdate, 100)
	progressDone := make(chan error, 1)

	go func() {
		progressDone <- tui.RunImportProgress(ctx, updates)
	}()

	tlsFlag, _ := cmd.Flags().GetBool("import-tls")
	startTLSFlag, _ := cmd.Flags().GetBool("import-starttls")
	// Use config values when the flags were not explicitly changed from defaults.
	if !cmd.Flags().Changed("import-tls") {
		tlsFlag = cfg.Import.TLS
	}
	if !cmd.Flags().Changed("import-starttls") {
		startTLSFlag = cfg.Import.StartTLS
	}

	client := imapclient.New(cfg.Import.Host, cfg.Import.Port, cfg.Import.Username, cfg.Import.Password, tlsFlag, startTLSFlag)

	if err := client.Connect(); err != nil {
		close(updates)
		return fmt.Errorf("connecting to target: %w", err)
	}
	defer client.Close()

	if cfg.Import.Google {
		// OAuth2 OAUTHBEARER authentication for Gmail/GSuite.
		accessToken, newRefresh, err := google.GetAccessToken(
			ctx,
			cfg.Import.OAuth2.ClientID,
			cfg.Import.OAuth2.ClientSecret,
			cfg.Import.OAuth2.RefreshToken,
			"",
		)
		if err != nil {
			close(updates)
			return fmt.Errorf("obtaining Google access token: %w", err)
		}
		if newRefresh != "" && newRefresh != cfg.Import.OAuth2.RefreshToken {
			cfg.Import.OAuth2.RefreshToken = newRefresh
			if cfgPath, err := config.DefaultConfigPath(); err == nil {
				_ = cfg.Save(cfgPath)
			}
		}
		if err := client.AuthenticateOAuth2(accessToken); err != nil {
			close(updates)
			return fmt.Errorf("authenticating with Google: %w", err)
		}
	} else {
		if err := client.Authenticate(); err != nil {
			close(updates)
			return fmt.Errorf("authenticating with target: %w", err)
		}
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
