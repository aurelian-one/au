package au

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
)

const (
	ConfigDirEnvironmentVariable    = "AU_DIRECTORY"
	WorkspaceUidEnvironmentVariable = "AU_WORKSPACE"
	DefaultConfigDir                = "$HOME/.au"
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

func ResolveWorkspaceUid(directory string, flagValue string) (string, error) {
	if flagValue == "" {
		slog.Debug("no workspace uid provided on the cli - falling back to $" + WorkspaceUidEnvironmentVariable)
		flagValue = os.Getenv(WorkspaceUidEnvironmentVariable)
	}
	if flagValue == "" {
		slog.Debug("no workspace uid provided on the environment - checking symlink")
		symlinkPath := filepath.Join(directory, "current")
		symlinkDest, err := os.Readlink(symlinkPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", errors.Wrap(err, "failed to check for symlink")
			}
		} else {
			base := strings.TrimSuffix(filepath.Base(symlinkDest), ".automerge")
			if _, err := ulid.Parse(base); err != nil {
				return "", errors.New("symlink points to an invalid workspace")
			}
			flagValue = base
		}
	}
	if flagValue != "" {
		slog.Debug("current workspace resolved", "workspace", flagValue)
		if _, err := os.Stat(filepath.Join(directory, flagValue+".automerge")); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", errors.Wrap(err, "failed to check target workspace")
			}
			return "", errors.New("resolved workspace does not exist")
		}
		return flagValue, nil
	}
	return "", nil
}
