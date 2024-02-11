package commentcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestComment(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	assert.NoError(t, err)
	var todoId string
	{
		openWs, err := s.OpenWorkspace(context.Background(), wsMeta.Id, true)
		assert.NoError(t, err)
		todo, err := openWs.CreateTodo(context.Background(), au.CreateTodoParams{Title: "something", CreatedBy: "Example <name@me.com>"})
		assert.NoError(t, err)
		todoId = todo.Id
		assert.NoError(t, openWs.Flush())
		assert.NoError(t, openWs.Close())
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, wsMeta.Id)
	ctx = context.WithValue(ctx, common.CurrentAuthorContextKey, "Example <email@me.com>")

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

	t.Run("create markdown on todo", func(t *testing.T) {
		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"create", todoId, "--markdown", "Something\nElse"}))
		var out map[string]interface{}
		assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
		assert.Equal(t, "Something\nElse", out["content"])
		assert.Equal(t, "text/markdown", out["media_type"])
		commentId := out["id"].(string)

		t.Run("list", func(t *testing.T) {
			buff.Reset()
			var out []map[string]interface{}
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list", todoId}))
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Len(t, out, 1)
			assert.Equal(t, "Something\nElse", out[0]["content"])
		})

		t.Run("get", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"get", todoId, commentId}))
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Equal(t, "Something\nElse", out["content"])
			assert.Equal(t, commentId, out["id"].(string))
			assert.Nil(t, out["updated_at"])
			assert.Nil(t, out["updated_by"])
		})

		t.Run("edit", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"edit", todoId, commentId, "--markdown", "New Content"}))
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Equal(t, "New Content", out["content"])
			assert.Equal(t, commentId, out["id"].(string))
			assert.NotNil(t, out["updated_at"])
			assert.NotNil(t, out["updated_by"])
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

	t.Run("create content from file on todo", func(t *testing.T) {
		_ = os.WriteFile(filepath.Join(td, "example.html"), []byte("hello world"), 0600)

		buff.Reset()
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"create", todoId, "--content", filepath.Join(td, "example.html")}))
		var out map[string]interface{}
		assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
		assert.Equal(t, "<11 bytes hidden>", out["content"])
		assert.Equal(t, "text/html; charset=utf-8; filename=\"example.html\"", out["media_type"])
		commentId := out["id"].(string)

		t.Run("get no raw", func(t *testing.T) {
			buff.Reset()
			assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"get", todoId, commentId}))
			assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
			assert.Equal(t, "<11 bytes hidden>", out["content"])
			assert.Equal(t, "text/html; charset=utf-8; filename=\"example.html\"", out["media_type"])
			assert.Equal(t, commentId, out["id"])
		})
	})

}

func TestCli_todo_authors(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	assert.NoError(t, err)

	var todoId string
	{
		openWs, err := s.OpenWorkspace(context.Background(), wsMeta.Id, true)
		assert.NoError(t, err)
		todo, err := openWs.CreateTodo(context.Background(), au.CreateTodoParams{Title: "something", CreatedBy: "Example <email@me.com>"})
		assert.NoError(t, err)
		todoId = todo.Id
		assert.NoError(t, openWs.Flush())
		assert.NoError(t, openWs.Close())
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, wsMeta.Id)
	ctx = context.WithValue(ctx, common.CurrentAuthorContextKey, "Example <email@me.com>")

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"create", todoId, "--markdown", "Comment 1"}))
	var outStruct map[string]interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	commentId := outStruct["id"].(string)
	assert.Equal(t, "Example <email@me.com>", outStruct["created_by"])
	assert.Equal(t, "text/markdown", outStruct["media_type"])
	assert.Nil(t, outStruct["updated_at"])
	assert.Nil(t, outStruct["updated_by"])

	ctx = context.WithValue(ctx, common.CurrentAuthorContextKey, "Example2 <email@me.com>")

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"edit", todoId, commentId, "--markdown", "Comment 2"}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.Equal(t, "Example <email@me.com>", outStruct["created_by"])
	assert.NotNil(t, outStruct["updated_at"])
	assert.Equal(t, "Example2 <email@me.com>", outStruct["updated_by"])
}
