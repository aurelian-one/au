package au

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_success(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		d, err := NewDirectoryStorage(filepath.Join(td, "some/path"))
		assert.NoError(t, err)
		assert.NotNil(t, d)
		assert.DirExists(t, filepath.Join(td, "some/path"))
	}
}

func TestNew_follow_symlink(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(td, "real"), 0755))
	require.NoError(t, os.Symlink(filepath.Join(td, "real"), filepath.Join(td, "virtual")))
	d, err := NewDirectoryStorage(filepath.Join(td, "virtual"))
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestNew_fail(t *testing.T) {
	td, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(td, "thing"), []byte{}, os.FileMode(0644)))

	d, err := NewDirectoryStorage(filepath.Join(td, "thing"))
	assert.EqualError(t, err, "provided path is not a directory")
	assert.Nil(t, d)
}

func newDirectoryStorage(t *testing.T) StorageProvider {
	td, err := os.MkdirTemp(os.TempDir(), "au")
	require.NoError(t, err)
	d, err := NewDirectoryStorage(filepath.Join(td, "thing"))
	require.NoError(t, err)
	d.(*directoryStorage).Logger = slog.Default().With("test", t.Name())
	t.Cleanup(func() {
		_ = os.RemoveAll(td)
	})
	return d
}

func TestCreate_success(t *testing.T) {
	s := newDirectoryStorage(t)

	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "something"})
	assert.NoError(t, err)
	assert.Equal(t, ws.Alias, "something")
	ulid.MustParse(ws.Id)
	assert.Less(t, time.Since(ws.CreatedAt), time.Minute)
	assert.Greater(t, int(ws.SizeBytes), 0)
	assert.Greater(t, 1000, int(ws.SizeBytes))

	content, err := os.ReadFile(filepath.Join(s.(*directoryStorage).Path, ws.Id+Suffix))
	if assert.NoError(t, err) {
		doc, err := automerge.Load(content)
		if assert.NoError(t, err) {

			out, err := automerge.As[*map[string]interface{}](doc.Root())
			assert.NoError(t, err)
			assert.Equal(t, &map[string]interface{}{
				"alias":      "something",
				"created_at": ws.CreatedAt,
				"todos":      map[string]interface{}{},
			}, out)
		}
	}
}

func TestCreate_fail_write(t *testing.T) {
	s := newDirectoryStorage(t)
	assert.NoError(t, os.Remove(s.(*directoryStorage).Path))
	_, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "something"})
	if assert.Error(t, err) {
		assert.ErrorContains(t, err, "failed to write workspace file: open")
		assert.ErrorContains(t, err, ".automerge.temp: no such file or directory")
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestGet_workspace(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "something"})
	if assert.NoError(t, err) {
		ws2, err := s.GetWorkspace(context.Background(), ws.Id)
		assert.NoError(t, err)
		assert.Equal(t, ws, ws2)
	}
}

func TestGetWorkspace_not_found(t *testing.T) {
	s := newDirectoryStorage(t)
	_, err := s.GetWorkspace(context.Background(), "hello")
	if assert.Error(t, err) {
		assert.ErrorContains(t, err, "failed to read workspace: open")
		assert.ErrorContains(t, err, "hello.automerge: no such file or directory")
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestListWorkspaces_empty(t *testing.T) {
	s := newDirectoryStorage(t)
	o, err := s.ListWorkspaces(context.Background())
	assert.NoError(t, err)
	assert.Len(t, o, 0)
}

func TestListWorkspaces_some(t *testing.T) {
	s := newDirectoryStorage(t)
	for i := 0; i < 10; i++ {
		_, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "something"})
		assert.NoError(t, err)
	}
	o, err := s.ListWorkspaces(context.Background())
	assert.NoError(t, err)
	assert.Len(t, o, 10)
}

func TestDeleteWorkspace_missing(t *testing.T) {
	s := newDirectoryStorage(t)
	err := s.DeleteWorkspace(context.Background(), ulid.Make().String())
	if assert.Error(t, err) {
		assert.ErrorContains(t, err, "failed to delete workspace: remove")
		assert.ErrorContains(t, err, ".automerge: no such file or directory")
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestGetCurrentWorkspace_none(t *testing.T) {
	s := newDirectoryStorage(t)
	u, err := s.GetCurrentWorkspace(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "", u)
}

func TestSetAndGetCurrentWorkspace(t *testing.T) {
	s := newDirectoryStorage(t)
	assert.NoError(t, s.SetCurrentWorkspace(context.Background(), "something"))
	u, err := s.GetCurrentWorkspace(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "something", u)
}

func TestImportWorkspace_bad_uid(t *testing.T) {
	s := newDirectoryStorage(t)
	_, err := s.ImportWorkspace(context.Background(), "something", automerge.New().Save())
	assert.EqualError(t, err, "invalid workspace id - expected a valid ulid")
}

func TestImportWorkspace_bad_bytes(t *testing.T) {
	s := newDirectoryStorage(t)
	_, err := s.ImportWorkspace(context.Background(), ulid.Make().String(), []byte("bananas"))
	assert.EqualError(t, err, "data is not an automerge document")
}

func TestImportWorkspace_bad_document(t *testing.T) {
	s := newDirectoryStorage(t)
	_, err := s.ImportWorkspace(context.Background(), ulid.Make().String(), automerge.New().Save())
	assert.EqualError(t, err, "automerge document 'alias' is KindVoid, expected KindStr")
}

func TestImportWorkspace_good_document(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "example"})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(s.(*directoryStorage).Path, ws.Id+Suffix))
	require.NoError(t, err)

	newId := ulid.Make().String()
	ws2, err := s.ImportWorkspace(context.Background(), newId, content)
	require.NoError(t, err)

	assert.NotEqual(t, ws.Id, ws2.Id)
	ws.Id = ""
	ws2.Id = ""
	assert.Equal(t, ws, ws2)
}

func TestOpenWorkspaceReadable_missing(t *testing.T) {
	s := newDirectoryStorage(t)
	_, err := s.OpenWorkspace(context.Background(), ulid.Make().String(), false)
	if assert.Error(t, err) {
		assert.ErrorContains(t, err, "failed to read workspace: open")
		assert.ErrorContains(t, err, ".automerge: no such file or directory")
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestOpenWorkspaceReadable_success(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "example"})
	require.NoError(t, err)

	wsp, err := s.OpenWorkspace(context.Background(), ws.Id, false)
	wsp2 := wsp.(*directoryStorageWorkspace)
	assert.Nil(t, wsp2.Unlocker)
	assert.Equal(t, wsp2.Metadata, *ws)
	assert.Equal(t, filepath.Join(s.(*directoryStorage).Path, ws.Id+Suffix), wsp2.Path)
	assert.NotNil(t, wsp2.Doc)
	assert.EqualError(t, wsp2.Flush(), "workspace is not locked for writing")
	assert.Nil(t, wsp2.Close())
}

func TestOpenWorkspaceWriteable_locked(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "example"})
	require.NoError(t, err)

	wsp, err := s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.NoError(t, err)
	defer wsp.Close()

	wsp2, err := s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.EqualError(t, err, "failed to lock the workspace for editing: it is already locked by another process")
	assert.Nil(t, wsp2)
}

func TestOpenWorkspaceWriteable_unlocked(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "example"})
	require.NoError(t, err)

	wsp, err := s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.NoError(t, err)
	assert.NoError(t, wsp.Close())

	wsp2, err := s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.NoError(t, err)
	assert.NoError(t, wsp2.Close())
}

func TestOpenWorkspace_flush_multiple(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, err := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "example"})
	require.NoError(t, err)

	wsp, err := s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.NoError(t, err)
	defer wsp.Close()

	assert.NoError(t, wsp.(*directoryStorageWorkspace).Doc.Doc.Path("a").Set("b"))
	assert.NoError(t, wsp.Flush())
	assert.NoError(t, wsp.(*directoryStorageWorkspace).Doc.Doc.Path("b").Set("c"))
	assert.NoError(t, wsp.Flush())
	assert.NoError(t, wsp.Close())

	wsp, err = s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.NoError(t, err)
	defer wsp.Close()

	v, _ := wsp.(*directoryStorageWorkspace).Doc.Doc.Path("a").Get()
	assert.Equal(t, "b", v.Str())
	v, _ = wsp.(*directoryStorageWorkspace).Doc.Doc.Path("b").Get()
	assert.Equal(t, "c", v.Str())
}
