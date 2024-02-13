package au

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	ConfigDirEnvironmentVariable    = "AU_DIRECTORY"
	WorkspaceUidEnvironmentVariable = "AU_WORKSPACE"
	AuthorEnvironmentVariable       = "AU_AUTHOR"
	EditorVariable                  = "AU_EDITOR"
	GlobalEditorVariable            = "EDITOR"
	DefaultConfigDir                = "$HOME/.au"
)

func ResolveConfigDirectory(flagValue string, getEnv func(string) string) (string, error) {
	if flagValue == "" {
		slog.Debug("no config directory provided on the cli - falling back to $" + ConfigDirEnvironmentVariable)
		flagValue = getEnv(ConfigDirEnvironmentVariable)
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

func ResolveWorkspaceUid(flagValue string, getEnv func(string) string) (string, error) {
	if flagValue == "" {
		slog.Debug("no workspace id provided on the cli - falling back to $" + WorkspaceUidEnvironmentVariable)
		flagValue = getEnv(WorkspaceUidEnvironmentVariable)
	}
	if flagValue == "" {
		slog.Debug("no workspace id provided on $" + WorkspaceUidEnvironmentVariable + " falling back to workspace file")
	}
	return flagValue, nil
}

func ResolveAuthor(flagValue string, getEnv func(string) string) (string, error) {
	if flagValue == "" {
		slog.Debug("no author provided on the cli - falling back to $" + AuthorEnvironmentVariable)
		flagValue = getEnv(AuthorEnvironmentVariable)
	}
	if flagValue == "" {
		slog.Debug("no author provided on $" + AuthorEnvironmentVariable + " falling back to author on workspace")
	}
	return flagValue, nil
}
