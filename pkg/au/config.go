package au

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	ConfigDirEnvironmentVariable = "AU_DIRECTORY"
	DefaultConfigDir             = "$HOME/.au"
)

func ResolveConfigDirectory(flagValue string) (string, error) {
	if flagValue == "" {
		slog.Debug("no config directory provided on the cli - falling back to $" + ConfigDirEnvironmentVariable)
		flagValue = os.Getenv(ConfigDirEnvironmentVariable)
	}
	if flagValue == "" {
		slog.Debug("no config directory provided on the environment - falling back to default")
		flagValue = DefaultConfigDir
	}
	flagValue = filepath.Clean(os.ExpandEnv(flagValue))
	if !filepath.IsAbs(flagValue) {
		slog.Debug("config directory is not absolute - resolving it")
		absValue, err := filepath.Abs(flagValue)
		if err != nil {
			return "", errors.Wrap(err, "failed to resolve directory")
		}
		flagValue = absValue
	}
	slog.Debug("config directory resolved", "dir", flagValue)
	return flagValue, nil
}