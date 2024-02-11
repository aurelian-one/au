package workspacecmd

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
	"github.com/aurelian-one/au/pkg/auws"
)

func executeAndResetCommand(ctx context.Context, cmd *cobra.Command, args []string) error {
	cmd.SetArgs(args)
	subCmd, err := cmd.ExecuteContextC(ctx)
	subCmd.SetContext(nil)
	return err
}

func TestCli_create_and_delete(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	s, _ := au.NewDirectoryStorage(td)

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, "")

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list"}))
	assert.Equal(t, "[]\n", buff.String())

	buff.Reset()
	assert.EqualError(t, executeAndResetCommand(ctx, Command, []string{"get"}), "current workspace not set")

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"init", "Example Workspace"}))
	var out struct {
		Id string `yaml:"id"`
	}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.NotEqual(t, "", out.Id)
	workspaceId := out.Id

	if ws, err := s.GetCurrentWorkspace(ctx); err != nil {
		assert.NoError(t, err)
	} else {
		assert.Equal(t, workspaceId, ws)
	}
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, workspaceId)

	buff.Reset()
	var outStruct map[string]interface{}
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"get"}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.Equal(t, workspaceId, outStruct["id"].(string))
	assert.Nil(t, outStruct["current_author"])

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"use", workspaceId}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.Equal(t, workspaceId, out.Id)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"list"}))
	var outSlice []interface{}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outSlice))
	assert.Len(t, outSlice, 1)
	assert.Equal(t, workspaceId, outSlice[0].(map[string]interface{})["id"])

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"set-author", "Example <name@email>"}))

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"get"}))
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &outStruct))
	assert.Equal(t, workspaceId, outStruct["id"].(string))
	assert.Equal(t, "Example <name@email>", outStruct["current_author"])

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"delete", workspaceId}))
	assert.Equal(t, "", buff.String())
}

func TestCli_serve(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	s, _ := au.NewDirectoryStorage(td)
	wsMeta, err := s.CreateWorkspace(context.Background(), au.CreateWorkspaceParams{Alias: "Example"})
	assert.NoError(t, err)
	workspaceId := wsMeta.Id

	ctx := context.Background()
	ctx = context.WithValue(ctx, common.StorageContextKey, s)
	ctx = context.WithValue(ctx, common.CurrentWorkspaceIdContextKey, wsMeta.Id)

	buff := new(bytes.Buffer)
	Command.SetOut(buff)
	Command.SetErr(buff)

	buff.Reset()
	assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"use", workspaceId}))
	var out struct {
		Id string `yaml:"id"`
	}
	assert.NoError(t, yaml.Unmarshal(buff.Bytes(), &out))
	assert.Equal(t, workspaceId, out.Id)

	buff.Reset()
	ctx, cancel := context.WithCancel(context.WithValue(ctx, common.ListenerRefContextKey, new(atomic.Value)))
	defer cancel()

	go func() {
		assert.NoError(t, executeAndResetCommand(ctx, Command, []string{"serve", "127.0.0.1:0"}))
	}()
	assert.Eventually(t, func() bool {
		return ctx.Value(common.ListenerRefContextKey).(*atomic.Value).Load() != nil
	}, time.Second*10, time.Millisecond*100)
	address := ctx.Value(common.ListenerRefContextKey).(*atomic.Value).Load().(net.Listener).Addr().String()

	c, err := NewClientWithResponses("http://" + address)
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
			req, _ := NewSynchroniseWorkspaceDocumentRequest("ws://"+address, workspaceId)

			conn, _, err := websocket.DefaultDialer.Dial(req.URL.String(), nil)
			assert.NoError(t, err)
			defer conn.Close()
			assert.NoError(t, auws.Sync(ctx, slog.Default(), conn, doc, true))
		})
	})

}
