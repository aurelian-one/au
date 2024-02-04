package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/cmd/au/workspacecmd"
	"github.com/aurelian-one/au/pkg/au"
	"github.com/aurelian-one/au/pkg/auws"
)

func TestCli_create_and_delete(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(au.ConfigDirEnvironmentVariable, td))

	buff := new(bytes.Buffer)
	rootCmd.SetOut(buff)
	rootCmd.SetErr(buff)
	rootCmd.SetArgs([]string{"workspace", "list"})
	assert.NoError(t, rootCmd.Execute())
	assert.Equal(t, "[]\n", buff.String())

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "get"})
	assert.EqualError(t, rootCmd.Execute(), "current workspace not set")

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "init", "Example Workspace"})
	assert.NoError(t, rootCmd.Execute())
	var out struct {
		Id string `yaml:"id"`
	}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.NotEqual(t, "", out.Id)
	workspaceId := out.Id

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "get"})
	var outStruct map[string]interface{}
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.Equal(t, workspaceId, outStruct["id"].(string))

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "use", workspaceId})
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.Equal(t, workspaceId, out.Id)

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "list"})
	assert.NoError(t, rootCmd.Execute())
	var outSlice []interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 1)
	assert.Equal(t, workspaceId, outSlice[0].(map[string]interface{})["id"])

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "delete", workspaceId})
	assert.NoError(t, rootCmd.Execute())
	assert.Equal(t, "", buff.String())
}

func TestCli_create_todos(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(au.ConfigDirEnvironmentVariable, td))

	buff := new(bytes.Buffer)
	rootCmd.SetOut(buff)
	rootCmd.SetErr(buff)
	rootCmd.SetArgs([]string{"workspace", "init", "Example Workspace"})
	assert.NoError(t, rootCmd.Execute())
	var out struct {
		Id string `yaml:"id"`
	}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.NotEqual(t, "", out.Id)
	workspaceId := out.Id

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "use", workspaceId})
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.Equal(t, workspaceId, out.Id)

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "list"})
	assert.NoError(t, rootCmd.Execute())
	var outSlice []interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 0)

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "create", "--title", "My todo", "--description", "Some longer description of the todo"})
	assert.NoError(t, rootCmd.Execute())
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
		"annotations": map[string]interface{}{},
	}, outStruct)

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "list"})
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 1)
	assert.Equal(t, todoId, outSlice[0].(map[string]interface{})["id"].(string))

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "get", todoId})
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.Equal(t, todoId, outStruct["id"].(string))

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "edit", "--title", "My todo 2", "--description", "Edited description", "--status", "closed", todoId})
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.NotNil(t, outStruct["created_at"].(time.Time))
	delete(outStruct, "created_at")
	assert.Equal(t, map[string]interface{}{
		"id":          todoId,
		"title":       "My todo 2",
		"description": "Edited description",
		"status":      "closed",
		"annotations": map[string]interface{}{},
	}, outStruct)

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "delete", todoId})
	assert.NoError(t, rootCmd.Execute())
	assert.Equal(t, "", buff.String())

	buff.Reset()
	rootCmd.SetArgs([]string{"todo", "get", todoId})
	assert.EqualError(t, rootCmd.Execute(), fmt.Sprintf("failed to get todo: todo with id '%s' does not exist", todoId))
}

func TestCli_serve(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(au.ConfigDirEnvironmentVariable, td))

	buff := new(bytes.Buffer)
	rootCmd.SetOut(buff)
	rootCmd.SetErr(buff)
	rootCmd.SetArgs([]string{"workspace", "init", "Example Workspace"})
	assert.NoError(t, rootCmd.Execute())
	var out struct {
		Id string `yaml:"id"`
	}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.NotEqual(t, "", out.Id)
	workspaceId := out.Id

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "use", workspaceId})
	assert.NoError(t, rootCmd.Execute())
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.Equal(t, workspaceId, out.Id)

	buff.Reset()
	rootCmd.SetArgs([]string{"workspace", "serve", "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), common.ListenerRefContextKey, new(atomic.Value)))
	defer cancel()

	go func() {
		assert.NoError(t, rootCmd.ExecuteContext(ctx))
	}()
	assert.Eventually(t, func() bool {
		return ctx.Value(common.ListenerRefContextKey).(*atomic.Value).Load() != nil
	}, time.Second*10, time.Millisecond*100)
	address := ctx.Value(common.ListenerRefContextKey).(*atomic.Value).Load().(net.Listener).Addr().String()

	c, err := workspacecmd.NewClientWithResponses("http://" + address)
	assert.NoError(t, err)

	t.Run("can list workspaces", func(t *testing.T) {
		resp, err := c.ListWorkspaceWithResponse(ctx)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Len(t, *resp.JSON200, 1)
		assert.Equal(t, workspaceId, (*resp.JSON200)[0].Id)
	})

	t.Run("can get workspace 404", func(t *testing.T) {
		resp, err := c.GetWorkspaceWithResponse(ctx, "unknown")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("can get workspace", func(t *testing.T) {
		resp, err := c.GetWorkspaceWithResponse(ctx, workspaceId)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, workspaceId, (*resp.JSON200).Id)
	})

	t.Run("can download workspace", func(t *testing.T) {
		resp, err := c.DownloadWorkspaceDocumentWithResponse(ctx, workspaceId)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		doc, err := automerge.Load(resp.Body)
		assert.NoError(t, err)

		t.Run("can sync workspace", func(t *testing.T) {
			req, _ := workspacecmd.NewSynchroniseWorkspaceDocumentRequest("ws://"+address, workspaceId)

			conn, _, err := websocket.DefaultDialer.Dial(req.URL.String(), nil)
			assert.NoError(t, err)
			defer conn.Close()
			assert.NoError(t, auws.Sync(ctx, slog.Default(), conn, doc, true))
		})
	})

}
