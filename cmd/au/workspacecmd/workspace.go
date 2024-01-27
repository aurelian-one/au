package workspacecmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

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
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if err := c.Init(); err != nil {
			return errors.Wrap(err, "failed to init directory")
		}
		uid, err := c.CreateNewWorkspace(args[0])
		if err != nil {
			return err
		}
		metadata, err := c.GetWorkspaceMetadata(uid)
		if err != nil {
			return err
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(metadata)
	},
}

var listCommand = &cobra.Command{
	Use:  "list",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		workspaceUids, err := c.ListWorkspaceUids()
		if err != nil {
			return errors.Wrap(err, "failed to list workspaces")
		}
		output := make([]*au.WorkspaceMetadata, 0, len(workspaceUids))
		for _, uid := range workspaceUids {
			metadata, err := c.GetWorkspaceMetadata(uid)
			if err != nil {
				return errors.Wrap(err, "failed to read metadata")
			}
			output = append(output, metadata)
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(output)
	},
}

var useCommand = &cobra.Command{
	Use:        "use <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if exists, err := c.DoesWorkspaceExist(args[0]); err != nil {
			return errors.Wrap(err, "failed to check for workspace")
		} else if !exists {
			return errors.New("workspace does not exist")
		}
		if err := c.ChangeCurrentWorkspaceUid(args[0]); err != nil {
			return errors.Wrap(err, "failed to change current workspace")
		}
		metadata, err := c.GetWorkspaceMetadata(args[0])
		if err != nil {
			return err
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(metadata)
	},
}

var deleteCommand = &cobra.Command{
	Use:        "delete <uid>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if exists, err := c.DoesWorkspaceExist(args[0]); err != nil {
			return errors.Wrap(err, "failed to check for workspace")
		} else if !exists {
			return errors.New("workspace does not exist")
		}
		confirmed, err := au.Confirm(fmt.Sprintf("Are you sure you want to delete workspace %s in directory %s?", args[0], c.Path), os.Stdout, os.Stdin)
		if err != nil {
			return errors.Wrap(err, "failed to confirm")
		}
		if !confirmed {
			return &common.ExitWithCode{Code: 1}
		}
		return errors.Wrap(c.DeleteWorkspace(args[0]), "failed to delete workspace")
	},
}

var syncServerCommand = &cobra.Command{
	Use:        "serve <localhost:80>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"address"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		return c.ListenAndServe(cmd.Context(), cmd.Flags().Arg(0))
	},
}

var syncClientCommand = &cobra.Command{
	Use:        "sync <ws://localhost:80>",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"address"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		if c.CurrentUid == "" {
			return errors.New("no current workspace set")
		}
		return c.ConnectAndSync(cmd.Context(), c.CurrentUid, cmd.Flags().Arg(0))
	},
}

var syncImportCommand = &cobra.Command{
	Use:        "sync-import <http://localhost:80>",
	Args:       cobra.ExactArgs(2),
	ArgAliases: []string{"address", "uid"},
	RunE: func(cmd *cobra.Command, args []string) error {
		c := cmd.Context().Value(common.ConfigDirectoryContextKey).(*au.ConfigDirectory)
		return c.ConnectAndImport(cmd.Context(), args[1], args[0])
	},
}

func init() {
	Command.AddCommand(
		initCommand,
		listCommand,
		useCommand,
		deleteCommand,
		syncServerCommand,
		syncClientCommand,
		syncImportCommand,
	)
}
