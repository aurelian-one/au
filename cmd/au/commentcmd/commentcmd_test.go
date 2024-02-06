package commentcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

func executeAndResetCommand(ctx context.Context, cmd *cobra.Command, args []string) error {
	cmd.SetArgs(args)
	subCmd, err := cmd.ExecuteContextC(ctx)
	subCmd.SetContext(nil)
	return err
}

func TestCommand(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	assert.NoError(t, err)
	var todoId string
	{
		openWs, err := s.OpenWorkspace(context.Background(), wsMeta.Id, true)
		assert.NoError(t, err)
		todo, err := openWs.CreateTodo(context.Background(), au.CreateTodoParams{Title: "something"})
		todoId = todo.Id
		assert.NoError(t, err)
		assert.NoError(t, openWs.Flush())
		assert.NoError(t, openWs.Close())
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, wsMeta.Id)

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	t.Run("list on unknown todo", func(t *testing.T) {
		buff.Reset()
		assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"list", "unknown"}), "todo with id 'unknown' does not exist")
	})

	t.Run("get on unknown todo", func(t *testing.T) {
		buff.Reset()
		assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"get", "unknown", "unknown"}), "todo with id 'unknown' does not exist")
	})

	t.Run("create on unknown todo", func(t *testing.T) {
		buff.Reset()
		assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"create", "unknown", "--markdown", "Hello"}), "todo with id 'unknown' does not exist")
	})

	t.Run("edit on unknown todo", func(t *testing.T) {
		buff.Reset()
		assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"edit", "unknown", "unknown"}), "todo with id 'unknown' does not exist")
	})

	t.Run("delete on unknown todo", func(t *testing.T) {
		buff.Reset()
		assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"delete", "unknown", "unknown"}), "todo with id 'unknown' does not exist")
	})

	t.Run("list on empty todo", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list", todoId}))
		assert.Equal(t, "[]\n", buff.String())
	})

	t.Run("create on todo", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"create", todoId, "--markdown", "Something\nElse"}))
		var out au.Comment
		assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
		assert.Equal(t, "Something\nElse", out.Content)
		assert.Equal(t, "text/markdown", out.MediaType)
		commentId := out.Id

		t.Run("list", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list", todoId}))
			var out []au.Comment
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Len(t, out, 1)
			assert.Equal(t, "Something\nElse", out[0].Content)
		})

		t.Run("get", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"get", todoId, commentId}))
			var out au.Comment
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Equal(t, "Something\nElse", out.Content)
			assert.Equal(t, commentId, out.Id)
		})

		t.Run("edit", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"edit", todoId, commentId, "--markdown", "New Content"}))
			var out au.Comment
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Equal(t, "New Content", out.Content)
			assert.Equal(t, commentId, out.Id)
		})

		t.Run("delete", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"delete", todoId, commentId}))
			assert.Equal(t, "", buff.String())
		})

		t.Run("get again", func(t *testing.T) {
			buff.Reset()
			assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"get", todoId, commentId}), fmt.Sprintf("comment with id '%s' does not exist", commentId))
		})
	})

}
