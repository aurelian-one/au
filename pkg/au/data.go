package au

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/automerge/automerge-go"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
)

type ConfigDirectory struct {
	// Path is the filepath to the config directory, usually the expanded and absolute form of $HOME/.au but it might
	// be a directory local to a target workspace file.
	Path string

	// CurrentUid is the current
	CurrentUid string
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

func (c *ConfigDirectory) DoesWorkspaceExist(uid string) (bool, error) {
	if stat, err := os.Stat(filepath.Join(c.Path, uid+".automerge")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to check for workspace")
	} else if stat.IsDir() {
		return false, errors.New("path is a directory")
	}
	return true, nil
}

func (c *ConfigDirectory) ChangeCurrentWorkspaceUid(uid string) error {
	if exists, err := c.DoesWorkspaceExist(uid); err != nil {
		return errors.Wrap(err, "failed to check for workspace")
	} else if !exists {
		return errors.New("target workspace does not exist")
	}
	if err := os.Remove(filepath.Join(c.Path, "current")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrap(err, "failed to remove old symlink")
	}
	if err := os.Symlink(filepath.Join(c.Path, uid+".automerge"), filepath.Join(c.Path, "current")); err != nil {
		return errors.Wrap(err, "failed to symlink")
	}
	slog.Info(fmt.Sprintf("Set new current workspace %s.", uid))
	return nil
}

func (c *ConfigDirectory) ListWorkspaceUids() ([]string, error) {
	entries, err := os.ReadDir(c.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workspaces")
	}
	workspaceUids := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".automerge" {
			name := strings.TrimSuffix(entry.Name(), ".automerge")
			if _, err := ulid.Parse(name); err == nil {
				workspaceUids = append(workspaceUids, name)
			}
		}
	}
	return workspaceUids, nil
}

func (c *ConfigDirectory) CreateNewWorkspace(alias string) (string, error) {
	proposedUid := ulid.Make().String()
	proposedAlias := strings.TrimSpace(alias)
	if len(proposedAlias) == 0 {
		return "", errors.Errorf("The workspace alias cannot be empty")
	}
	workspacePath := filepath.Join(c.Path, proposedUid+".automerge")
	doc := automerge.New()
	_ = doc.Path("alias").Set(proposedAlias)
	if err := doc.Path("todos").Set(&automerge.Map{}); err != nil {
		return "", errors.Wrap(err, "failed to setup todos map")
	}
	changeHash, err := doc.Commit("Init", automerge.CommitOptions{AllowEmpty: true})
	if err != nil {
		return "", errors.Wrap(err, "failed to commit")
	}
	saved := doc.Save()
	if err := os.WriteFile(workspacePath, saved, os.FileMode(0600)); err != nil {
		return "", errors.Wrap(err, "failed to write")
	}
	if err := c.ChangeCurrentWorkspaceUid(proposedUid); err != nil {
		return "", err
	}
	slog.Info(fmt.Sprintf("Initialised new workspace '%s' (%s@%s) and set it as default.", proposedAlias, proposedUid, changeHash))
	return proposedUid, nil
}

func (c *ConfigDirectory) DeleteWorkspace(uid string) error {
	if c.CurrentUid == uid {
		if err := os.Remove(filepath.Join(c.Path, "current")); err != nil {
			return errors.Wrap(err, "failed to remove symlink")
		}
	}
	if err := os.Remove(filepath.Join(c.Path, uid+".automerge")); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove workspace")
	}
	slog.Info(fmt.Sprintf("Deleted workspace %s.", uid))
	return nil
}
