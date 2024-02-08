package todocmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
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

func TestCli_create_todos(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	assert.NoError(t, err)

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, wsMeta.Id)

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list"}))
	var outSlice []interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 0)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{
		"create", "--title", "My todo", "--description", "Some longer description of the todo",
		"--annotation", "about:blank#example=42", "--annotation", "about:blank#another=something else",
	}))
	var outStruct map[string]interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.NotNil(t, outStruct["created_at"].(time.Time))
	delete(outStruct, "created_at")
	todoId := outStruct["id"].(string)
	_, err = ulid.Parse(todoId)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"id":          todoId,
		"title":       "My todo",
		"description": "Some longer description of the todo",
		"status":      "open",
		"annotations": map[string]interface{}{
			"about:blank#example": "42",
			"about:blank#another": "something else",
		},
		"comment_count": 0,
	}, outStruct)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list"}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 1)
	assert.Equal(t, todoId, outSlice[0].(map[string]interface{})["id"].(string))

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"get", todoId}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.Equal(t, todoId, outStruct["id"].(string))

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{
		"edit", "--title", "My todo 2", "--description", "Edited description", "--status", "closed", todoId,
		"--annotation", "about:blank#example=", "--annotation", "about:blank#example2=13",
	}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.NotNil(t, outStruct["created_at"].(time.Time))
	delete(outStruct, "created_at")
	assert.NotNil(t, outStruct["updated_at"].(time.Time))
	delete(outStruct, "updated_at")
	assert.Equal(t, map[string]interface{}{
		"id":          todoId,
		"title":       "My todo 2",
		"description": "Edited description",
		"status":      "closed",
		"annotations": map[string]interface{}{
			"about:blank#another":  "something else",
			"about:blank#example2": "13",
		},
		"comment_count": 0,
	}, outStruct)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"delete", todoId}))
	assert.Equal(t, "", buff.String())

	buff.Reset()
	assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"get", todoId}), fmt.Sprintf("failed to get todo: todo with id '%s' does not exist", todoId))
}

func TestCli_list_ranking(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	assert.NoError(t, err)

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, wsMeta.Id)

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	buff.Reset()

	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"create", "--title", "Todo None"}))
	for _, r := range []string{
		"0", "0", "1", "5", "2", "0",
	} {
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{
			"create", "--title", "Todo " + r, "--annotation", "https://aurelian.one/annotations/rank=" + r,
		}))
	}
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"create", "--title", "Todo None"}))

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list"}))
	var outSlice []map[string]interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 8)
	titles := make([]string, 0)
	for _, todo := range outSlice {
		titles = append(titles, todo["title"].(string))
	}
	assert.Equal(t, []string{"Todo 5", "Todo 2", "Todo 1"}, titles[:3])
}
