package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/aurelian-one/au/cmd/au/commentcmd"
	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/cmd/au/devcmd"
	"github.com/aurelian-one/au/cmd/au/todocmd"
	"github.com/aurelian-one/au/cmd/au/workspacecmd"
	"github.com/aurelian-one/au/pkg/au"
)

var rootCmd = &cobra.Command{
	Use:   "au",
	Short: "au is the core CLI for interacting with aurelian task management",
	Long: strings.TrimSpace(`
A core CLI for interacting with aurelian task management workspaces manually in single-operation mode.

This CLI can be used for simple human tasks or be called by other processes in order to accomplish machine-driven goals 🤖. Development and debug commands are also provided.

Find more information at: https://github.com/aurelian-one/au`),
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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information of the au binary",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		intTimestamp, _ := strconv.Atoi(au.CommitTimestamp)
		_, _ = fmt.Fprintf(os.Stdout, "%s (%s @ %s)\n", au.Version, au.Commit, time.Unix(int64(intTimestamp), 0))
	},
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
	directoryStorage, err := au.NewDirectoryStorage(directoryValue)
	if err != nil {
		return err
	}
	workspaceValue, err := cmd.Flags().GetString(workspaceFlag)
	if err != nil {
		return err
	}
	workspaceValue, err = au.ResolveWorkspaceUid(workspaceValue)
	if err != nil {
		return err
	}
	if workspaceValue == "" {
		if r, err := directoryStorage.GetCurrentWorkspace(cmd.Context()); err != nil {
			return err
		} else {
			workspaceValue = r
		}
	}
	cmd.SetContext(context.WithValue(cmd.Context(), common.StorageContextKey, directoryStorage))
	cmd.SetContext(context.WithValue(cmd.Context(), common.CurrentWorkspaceIdContextKey, workspaceValue))
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
The path of the config directory to operate in. If no value is provided, this will fallback to $%s before falling back to %s".`,
			au.ConfigDirEnvironmentVariable, au.DefaultConfigDir,
		)),
	)
	rootCmd.PersistentFlags().String(
		"current-workspace", "",
		strings.TrimSpace(fmt.Sprintf(`
The uid of the target workspace to operate in. If no value is provided, this will fallback to $%s before falling back to 'current' file".`,
			au.WorkspaceUidEnvironmentVariable,
		)),
	)

	rootCmd.AddGroup(&cobra.Group{Title: "Core", ID: "core"})
	rootCmd.AddCommand(
		workspacecmd.Command,
		todocmd.Command,
		commentcmd.Command,
		devcmd.Command,
		versionCmd,
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
