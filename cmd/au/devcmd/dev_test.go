package devcmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

func executeAndResetCommand(ctx context.Context, cmd *cobra.Command, args []string) error {
	cmd.SetArgs(args)
	subCmd, err := cmd.ExecuteContextC(ctx)
	subCmd.SetContext(nil)
	subCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
	})
	return err
}

func TestFakeData(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	workspaceId := wsMeta.Id
	assert.NoError(t, err)

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, workspaceId)
	ctx = context.WithValue(ctx, common.CurrentAuthorContextKey, "Example <email@me.com>")

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	t.Run("generate fake data", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"fake-data", "--num", "10"}))
		workspaceId = strings.TrimSpace(buff.String())
		ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, workspaceId)
	})

	t.Run("generate fake data in the past", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"fake-data", "--num", "4", "--backtrack", "4"}))
		workspaceId = strings.TrimSpace(buff.String())
		ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, workspaceId)
	})

	t.Run("generate fake data in the past again", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"fake-data", "--num", "4", "--backtrack", "4"}))
		workspaceId = strings.TrimSpace(buff.String())
		ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, workspaceId)
	})

	t.Run("generate fake data", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"fake-data", "--num", "10"}))
		workspaceId = strings.TrimSpace(buff.String())
		ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, workspaceId)
	})

	t.Run("can dump history", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"history"}))
		var out []interface{}
		assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
		assert.Greater(t, len(out), 10)
	})

	t.Run("can dump history as dot", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"history-dot"}))
		assert.NotEmpty(t, len(strings.TrimSpace(buff.String())))
	})

	t.Run("can dump", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"history"}))
		var out []interface{}
		assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
		assert.Greater(t, len(out), 0)
	})
}
