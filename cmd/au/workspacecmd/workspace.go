package workspacecmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/automerge/automerge-go"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use: "workspace",
}

var initCommand = &cobra.Command{
	Use:        "init <alias>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"alias"},
	RunE: func(cmd *cobra.Command, args []string) error {
		proposedUid := ulid.Make().String()
		proposedAlias := strings.TrimSpace(args[0])
		if len(proposedAlias) == 0 {
			return errors.Errorf("The workspace alias cannot be empty")
		}
		configDir, _ := cmd.Flags().GetString("directory")
		c := &au.ConfigDirectory{Path: configDir}
		confirmed, err := au.Confirm(fmt.Sprintf("Are you sure you want to create workspace %s '%s' in directory %s?", proposedUid, proposedAlias, configDir), os.Stdout, os.Stdin)
		if err != nil {
			return errors.Wrap(err, "failed to confirm")
		}
		if !confirmed {
			return &common.ExitWithCode{Code: 1}
		}
		if err := c.Init(); err != nil {
			return errors.Wrap(err, "failed to init directory")
		}
		workspacePath := filepath.Join(c.Path, proposedUid+".automerge")
		doc := automerge.New()
		_ = doc.Path("alias").Set(proposedAlias)
		if err := doc.Path("todos").Set(&automerge.Map{}); err != nil {
			return errors.Wrap(err, "failed to setup todos map")
		}
		changeHash, err := doc.Commit("Init", automerge.CommitOptions{AllowEmpty: true})
		if err != nil {
			return errors.Wrap(err, "failed to commit")
		}
		saved := doc.Save()
		if err := os.WriteFile(workspacePath, saved, os.FileMode(0600)); err != nil {
			return errors.Wrap(err, "failed to write")
		}
		if err := os.Remove(filepath.Join(c.Path, "current")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "failed to remove old symlink")
		}
		if err := os.Symlink(workspacePath, filepath.Join(c.Path, "current")); err != nil {
			return errors.Wrap(err, "failed to symlink")
		}
		slog.Info(fmt.Sprintf("Initialised new workspace '%s' (%s@%s) and set it as default", proposedAlias, proposedUid, changeHash))
		return nil
	},
}

var listCommand = &cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, _ := cmd.Flags().GetString("directory")
		c := &au.ConfigDirectory{Path: configDir}

		currentWorkspaceUid, workspaceUids := "", make([]string, 0)
		if exists, err := c.Exists(); err != nil {
			return errors.Wrap(err, "failed to check directory")
		} else if exists {
			entries, err := os.ReadDir(c.Path)
			if err != nil {
				return errors.Wrap(err, "failed to list workspaces")
			}
			for _, entry := range entries {
				if !entry.IsDir() && filepath.Ext(entry.Name()) == ".automerge" {
					name := strings.TrimSuffix(entry.Name(), ".automerge")
					if _, err := ulid.Parse(name); err == nil {
						workspaceUids = append(workspaceUids, name)
					}
				} else if entry.Type() == os.ModeSymlink && entry.Name() == "current" {
					dest, err := os.Readlink(filepath.Join(c.Path, entry.Name()))
					if err != nil {
						return errors.Wrap(err, "failed to follow link")
					}
					currentWorkspaceUid = strings.TrimSuffix(filepath.Base(dest), ".automerge")
				}
			}
		}

		for _, uid := range workspaceUids {
			var suff string
			if uid == currentWorkspaceUid {
				suff = " (current)"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", uid, suff)
		}
		return nil
	},
}

var useCommand = &cobra.Command{
	Use: "use",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var deleteCommand = &cobra.Command{
	Use: "delete",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {

	Command.AddCommand(
		initCommand,
		listCommand,
		useCommand,
		deleteCommand,
	)
}
