package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/cmd/au/devcmd"
	"github.com/aurelian-one/au/cmd/au/todocmd"
	"github.com/aurelian-one/au/cmd/au/workspacecmd"
	"github.com/aurelian-one/au/pkg/au"
)

// Tree
//
// au init [alias] [flags..] --as=    - ensures the file tree exists and creates a new doc with a new uuid associated with it as well as the alias.
//
// au default [alias/id] --as=         - sets the given id to the default (this can be override by env var)
//
// au create "Do the thing" --description= --status= --output= --label=x=y
//
// au comment <number> --content-type= --content=foo --edit (open the content in $EDITOR and then save that in the comment)
//
// au list [flags] --output [filters]      - list items
//
// au get <number/title> --output
//
// au edit <number> --status --description ....   - patch aspects of the ticket --edit (open the title followed by --- and description in $EDITOR)
//
// au sync server                 - listen and wait for a peer to sync with
//
// au sync client                 - connect to address and sync and exit when complete
//
// au dev dump                  - dump the entire contents to json
//
// au dev import [alias]        - import content produced by dump and build a new document with it
//
// au dev merge --file
//
// one thing to work out is how we do locking of the au file :thinking: looks like most folks go with https://github.com/gofrs/flock

var rootCmd = &cobra.Command{
	Use:   "au",
	Short: "au is the core CLI for interacting with aurelian task management",
	Long: `A core CLI for interacting with aurelian task management workspaces manually in single-operation mode.
This CLI can be used for simple human tasks or be called by other processes in order to accomplish machine-driven
goals 🤖. Development and debug commands are also provided.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := setupLogger(cmd); err != nil {
			return err
		}
		if err := resolveConfigDirectoryAndWorkspace(cmd, "directory", "current-workspace"); err != nil {
			return err
		}
		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func setupLogger(cmd *cobra.Command) error {
	handlerOptions := &slog.HandlerOptions{AddSource: false, Level: slog.LevelError}
	if debugValue, err := cmd.Flags().GetInt("debug"); err != nil {
		return err
	} else if debugValue == 1 {
		handlerOptions.Level = slog.LevelDebug
	} else if debugValue == 2 {
		handlerOptions.AddSource = true
		handlerOptions.Level = slog.LevelDebug
	} else if debugValue != 0 {
		return fmt.Errorf("debug value must be 0, 1, or 2")
	}
	slogHandler := slog.NewTextHandler(cmd.ErrOrStderr(), handlerOptions)
	slogLogger := slog.New(slogHandler)
	slog.SetDefault(slogLogger)
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("debug logging is enabled")
	}
	return nil
}

func resolveConfigDirectoryAndWorkspace(cmd *cobra.Command, directoryFlag string, workspaceFlag string) error {
	directoryValue, err := cmd.Flags().GetString(directoryFlag)
	if err != nil {
		return err
	}
	directoryValue, err = au.ResolveConfigDirectory(directoryValue)
	if err != nil {
		return err
	}
	workspaceValue, err := cmd.Flags().GetString(workspaceFlag)
	if err != nil {
		return err
	}
	workspaceValue, err = au.ResolveWorkspaceUid(directoryValue, workspaceValue)
	if err != nil {
		return err
	}
	cmd.SetContext(context.WithValue(cmd.Context(), common.ConfigDirectoryContextKey, &au.ConfigDirectory{
		Path:       directoryValue,
		CurrentUid: workspaceValue,
	}))
	return nil
}

func init() {
	rootCmd.PersistentFlags().IntP(
		"debug", "d", 0,
		"Enable debug logging by setting this value > 0. Valid values are 0, 1, or 2.",
	)

	rootCmd.PersistentFlags().String(
		"directory", "",
		strings.TrimSpace(fmt.Sprintf(`
The path of the config directory to operate in. If no value is provided, this will fallback to $%s before 
falling back to %s".`, au.ConfigDirEnvironmentVariable, au.DefaultConfigDir)),
	)
	rootCmd.PersistentFlags().String(
		"current-workspace", "",
		strings.TrimSpace(fmt.Sprintf(`
The uid of the target workspace to operate in. If no value is provided, this will fallback to $%s before 
falling back to current symlink".`, au.WorkspaceUidEnvironmentVariable)),
	)

	rootCmd.AddCommand(
		workspacecmd.Command,
		todocmd.Command,
		devcmd.Command,
	)
}

func main() {
	if err := mainInner(); err != nil && !errors.Is(err, flag.ErrHelp) {
		if v, _ := rootCmd.Flags().GetInt("debug"); v >= 2 {
			_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(), "%+v\n", err)
		}
		if ee := new(common.ExitWithCode); errors.As(err, &ee) {
			os.Exit(ee.Code)
		} else {
			_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func mainInner() error {
	return rootCmd.Execute()
}
