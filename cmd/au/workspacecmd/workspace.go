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
		_, err := c.CreateNewWorkspace(args[0])
		if err != nil {
			return err
		}
		return nil
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
		return yaml.NewEncoder(os.Stdout).Encode(output)
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
		return errors.Wrap(c.ChangeCurrentWorkspaceUid(args[0]), "failed to change current workspace")
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

func init() {
	Command.AddCommand(
		initCommand,
		listCommand,
		useCommand,
		deleteCommand,
	)
}
