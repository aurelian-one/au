package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func executeAndResetCommand(ctx context.Context, cmd *cobra.Command, args []string) error {
	cmd.SetArgs(args)
	subCmd, err := cmd.ExecuteContextC(ctx)
	subCmd.SetContext(nil)
	return err
}

func TestRoot(t *testing.T) {
	ctx := context.Background()

	buff := new(bytes.Buffer)
	rootCmd.SetOut(buff)
	rootCmd.SetErr(buff)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, rootCmd, []string{"--help"}))
	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, rootCmd, []string{"workspace", "--help"}))
	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, rootCmd, []string{"todo", "--help"}))
	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, rootCmd, []string{"comment", "--help"}))
}
