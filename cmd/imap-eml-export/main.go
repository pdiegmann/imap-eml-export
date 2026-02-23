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
	Short: "Export and import IMAP mailboxes as EML files",
	Long: `imap-eml-export connects to any IMAP server and exports every email in every
folder to local .eml files, preserving the complete folder hierarchy. It can
also import those files back into a (different) IMAP server — making it ideal
for backups, migrations, and archiving.

Configuration is read from (in order of priority):
  1. CLI flags (--export-host, --import-username, …)
  2. Environment variables (IMAP_EXPORT_HOST, IMAP_IMPORT_USERNAME, …)
  3. Config file (default: ~/.config/imap-eml-export/config.toml)
  4. Built-in defaults (port 993, TLS enabled, output ./output)

The config file uses separate [export] and [import] sections so source and
target accounts can live in one file. See config.example.toml for a fully
annotated template.

Gmail / Google Workspace users: pass --google (or set google = true in the
config) to authenticate via OAuth2 — no app-password required.`,
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export emails from an IMAP server to local .eml files",
	Long: `Connect to an IMAP server and download every message in every folder to the
local file system as individual .eml files.

Folder hierarchy is preserved: an IMAP folder "Work/ProjectA" becomes the
directory Work/ProjectA/ under the output directory.

File names follow the pattern:
  {sequence}_{YYYY-MM-DD}_{sanitized-subject}.eml
  e.g.  00001_2024-01-15_hello-world.eml

If no server credentials are found in the config file or environment, an
interactive setup wizard starts automatically and saves the config for future
runs.

Gmail / Google Workspace users: use --google instead of --export-password.
The first run opens a browser-based sign-in flow; the token is cached locally
so subsequent runs need no interaction.`,
	Example: `  # Minimal: let the wizard guide you
  imap-eml-export export

  # Provide credentials on the command line
  imap-eml-export export --export-host imap.example.com \
      --export-username me@example.com --export-password secret \
      --output ./backup

  # Gmail / Google Workspace via OAuth2
  imap-eml-export export --google --export-username me@gmail.com

  # Non-TLS / STARTTLS server
  imap-eml-export export --export-host mail.example.com --export-port 143 \
      --export-tls=false --export-starttls

  # Use a custom config file
  imap-eml-export export --config ~/my-config.toml`,
	RunE: runExport,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import .eml files from a local directory into an IMAP server",
	Long: `Walk a local directory tree of .eml files (as produced by the export command)
and upload every message to the corresponding IMAP mailbox on the target
server, recreating the original folder structure.

The input directory defaults to the value of import.input_dir in the config
file, falling back to export.output_dir. Override it with --input.

Gmail / Google Workspace users: use --google instead of --import-password.`,
	Example: `  # Import from the default output directory into a new server
  imap-eml-export import --import-host imap.newserver.com \
      --import-username me@newserver.com --import-password secret

  # Specify a custom input directory
  imap-eml-export import --input ./my-backup \
      --import-host imap.example.com --import-username me@example.com \
      --import-password secret

  # Import into Gmail / Google Workspace via OAuth2
  imap-eml-export import --google --import-username dest@gmail.com \
      --input ./backup`,
	RunE: runImport,
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update imap-eml-export to the latest release",
	Long: `Check GitHub for a newer release of imap-eml-export. If one is found, download
and replace the current binary in-place.

The update is downloaded from the official GitHub release page:
  https://github.com/pdiegmann/imap-eml-export/releases`,
	Example: `  imap-eml-export update`,
	RunE:    runUpdate,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the current version",
	Long:  `Print the version string that was embedded at build time.`,
	Example: `  imap-eml-export version`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("imap-eml-export %s\n", version)
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "config file path (default: ~/.config/imap-eml-export/config.toml)")
	rootCmd.PersistentFlags().String("log-file", "", "write log output to this file instead of stderr")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose/informational output")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug output (very verbose)")

	// export command flags – these override the [export] config section.
	exportCmd.Flags().String("export-host", "", "IMAP hostname or IP of the source server (e.g. imap.example.com)")
	exportCmd.Flags().Int("export-port", 0, "IMAP port of the source server (default: 993 for TLS, 143 for plain/STARTTLS)")
	exportCmd.Flags().StringP("export-username", "u", "", "login username for the source IMAP server (usually your email address)")
	exportCmd.Flags().StringP("export-password", "p", "", "login password for the source IMAP server (use an App Password for Gmail)")
	exportCmd.Flags().StringP("output", "o", "", "directory where exported .eml files are written (default: ./output)")
	exportCmd.Flags().Bool("export-tls", true, "use implicit TLS (IMAPS) for the source connection — recommended")
	exportCmd.Flags().Bool("export-starttls", false, "upgrade a plain connection to TLS via STARTTLS (use with --export-tls=false)")
	exportCmd.Flags().Bool("google", false, "sign in with Google OAuth2 instead of a password — works with Gmail and Google Workspace; sets host/port/TLS automatically")
	exportCmd.Flags().BoolP("yes", "y", false, "skip all confirmation prompts")

	// import command flags – these override the [import] config section.
	importCmd.Flags().String("import-host", "", "IMAP hostname or IP of the target server (e.g. imap.example.com)")
	importCmd.Flags().Int("import-port", 0, "IMAP port of the target server (default: 993 for TLS, 143 for plain/STARTTLS)")
	importCmd.Flags().StringP("import-username", "u", "", "login username for the target IMAP server (usually your email address)")
	importCmd.Flags().StringP("import-password", "p", "", "login password for the target IMAP server (use an App Password for Gmail)")
	importCmd.Flags().StringP("input", "i", "", "directory containing exported .eml files to upload (default: import.input_dir or export.output_dir from config)")
	importCmd.Flags().Bool("import-tls", true, "use implicit TLS (IMAPS) for the target connection — recommended")
	importCmd.Flags().Bool("import-starttls", false, "upgrade a plain connection to TLS via STARTTLS (use with --import-tls=false)")
	importCmd.Flags().Bool("google", false, "sign in with Google OAuth2 instead of a password — works with Gmail and Google Workspace; sets host/port/TLS automatically")

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
