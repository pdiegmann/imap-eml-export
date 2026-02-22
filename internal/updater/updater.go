package updater

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/creativeprojects/go-selfupdate"
)

const githubRepo = "pdiegmann/imap-eml-export"

// CheckUpdate checks whether a newer release is available.
// Returns the latest version string, a bool indicating if an update exists,
// and any error encountered.
func CheckUpdate(currentVersion string) (string, bool, error) {
	updater := selfupdate.DefaultUpdater()
	release, found, err := updater.DetectLatest(context.Background(), selfupdate.ParseSlug(githubRepo))
	if err != nil {
		return "", false, fmt.Errorf("detecting latest release: %w", err)
	}
	if !found {
		return "", false, nil
	}
	latest := release.Version()
	if release.Equal(currentVersion) {
		return latest, false, nil
	}
	return latest, true, nil
}

// DoUpdate downloads and applies the latest release, replacing the running binary.
func DoUpdate(currentVersion string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	updater := selfupdate.DefaultUpdater()
	_, err = updater.UpdateSelf(context.Background(), currentVersion, selfupdate.ParseSlug(githubRepo))
	if err != nil {
		return fmt.Errorf("updating %s: %w", executable, err)
	}
	return nil
}

// OSArch returns the current OS/architecture string.
func OSArch() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
