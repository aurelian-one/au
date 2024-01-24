package au

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type ConfigDirectory struct {
	// Path is the filepath to the config directory, usually the expanded and absolute form of $HOME/.au but it might
	// be a directory local to a target workspace file.
	Path string
}

func NewConfigDirectory(path string) (*ConfigDirectory, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("config directory path must be a clean absolute path")
	}
	return &ConfigDirectory{Path: path}, nil
}

// Exists returns whether the target config directory exists
func (c *ConfigDirectory) Exists() (bool, error) {
	stat, err := os.Stat(c.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("config directory does not exist", "dir", c.Path)
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check directory '%s'", c.Path)
	}
	if !stat.IsDir() {
		return false, errors.Errorf("path '%s' exists but is not a directory", c.Path)
	}
	slog.Debug("config directory exists", "dir", c.Path)
	return true, nil
}

func (c *ConfigDirectory) Init() error {
	if exists, err := c.Exists(); err != nil {
		return err
	} else if !exists {
		slog.Debug("config directory does not exist - creating it", "dir", c.Path)
		return errors.Wrap(os.MkdirAll(c.Path, os.FileMode(0700)), "failed to init config directory")
	}
	slog.Debug("config directory does not exist - asserting permissions", "dir", c.Path)
	return errors.Wrap(os.Chmod(c.Path, os.FileMode(0700)), "failed to set permissions on config directory")
}
