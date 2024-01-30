package au

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/gofrs/flock"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
)

const Suffix = ".automerge"

type directoryStorage struct {
	Path   string
	Logger *slog.Logger
}

func NewDirectoryStorage(path string) (StorageProvider, error) {
	if stat, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(path, os.FileMode(0755)); err != nil {
				return nil, errors.Wrap(err, "failed to create storage directory")
			}

		}
	} else if !stat.IsDir() {
		return nil, errors.New("provided path is not a directory")
	}
	return &directoryStorage{Path: path, Logger: slog.Default()}, nil
}

func (d *directoryStorage) ListWorkspaces(ctx context.Context) ([]WorkspaceMeta, error) {
	entries, err := os.ReadDir(d.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workspaces")
	}
	workspaceUids := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == Suffix {
			name := strings.TrimSuffix(entry.Name(), Suffix)
			if _, err := ulid.Parse(name); err == nil {
				workspaceUids = append(workspaceUids, name)
			}
		}
	}
	output := make([]WorkspaceMeta, len(workspaceUids))
	for i, uid := range workspaceUids {
		inner, err := d.GetWorkspace(ctx, uid)
		if err != nil {
			return nil, errors.Wrapf(err, "%s: failed to get workspace", uid)
		}
		output[i] = *inner
	}
	return output, nil
}

func (d *directoryStorage) GetWorkspace(ctx context.Context, id string) (*WorkspaceMeta, error) {
	if ws, err := d.OpenWorkspace(ctx, id, false); err != nil {
		return nil, err
	} else {
		return &ws.(*directoryStorageWorkspace).Metadata, nil
	}
}

func (d *directoryStorage) CreateWorkspace(ctx context.Context, params CreateWorkspaceParams) (*WorkspaceMeta, error) {
	var chosenId string
	for i := 0; i < 20; i++ {
		proposedId := ulid.Make().String()
		if _, err := os.Stat(filepath.Join(d.Path, proposedId+Suffix)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				chosenId = proposedId
				break
			}
			return nil, errors.Wrap(err, "failed to state files in workspace directory")
		}
	}
	if chosenId == "" {
		return nil, errors.New("failed to calculate unique workspace id")
	}

	doc := automerge.New()
	_ = doc.Path("alias").Set(params.Alias)
	createdAt := time.Now().UTC().Round(time.Second).Local()
	_ = doc.Path("created_at").Set(createdAt)
	_ = doc.Path("todos").Set(automerge.NewMap())

	content := doc.Save()
	path := filepath.Join(d.Path, chosenId+Suffix)
	tempPath := path + ".temp"
	if err := os.WriteFile(tempPath, content, os.FileMode(0644)); err != nil {
		return nil, errors.Wrap(err, "failed to write workspace file")
	}
	if err := os.Rename(tempPath, path); err != nil {
		return nil, errors.Wrap(err, "failed to move workspace file to target")
	}

	return &WorkspaceMeta{
		Id:        chosenId,
		Alias:     params.Alias,
		CreatedAt: createdAt,
		SizeBytes: int64(len(content)),
	}, nil
}

func (d *directoryStorage) DeleteWorkspace(ctx context.Context, id string) error {
	if err := os.Remove(filepath.Join(d.Path, id+Suffix)); err != nil {
		return errors.Wrapf(err, "failed to delete workspace")
	}
	return nil
}

func (d *directoryStorage) GetCurrentWorkspace(ctx context.Context) (string, error) {
	if raw, err := os.ReadFile(filepath.Join(d.Path, "current")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", errors.Wrap(err, "failed to read current")
	} else {
		return strings.TrimSpace(string(raw)), nil
	}
}

func (d *directoryStorage) SetCurrentWorkspace(ctx context.Context, id string) error {
	path := filepath.Join(d.Path, "current")
	tempPath := path + ".temp"

	if err := os.WriteFile(tempPath, []byte(id), os.FileMode(0644)); err != nil {
		return errors.Wrapf(err, "failed to write current")
	}
	if err := os.Rename(tempPath, path); err != nil {
		return errors.Wrap(err, "failed to rename current pointer")
	}
	return nil
}

func (d *directoryStorage) OpenWorkspace(ctx context.Context, id string, writeable bool) (WorkspaceProvider, error) {
	path := filepath.Join(d.Path, id+Suffix)

	var unlocker func()
	if writeable {
		locker := flock.New(path + ".lock")
		if locked, err := locker.TryLock(); err != nil {
			return nil, errors.Wrap(err, "failed to lock the workspace for writing")
		} else if !locked {
			return nil, errors.New("failed to lock the workspace for editing: it is already locked by another process")
		}
		unlocker = func() {
			_ = locker.Unlock()
		}
	}
	defer func() {
		if unlocker != nil {
			unlocker()
		}
	}()

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read workspace")
	}
	doc, err := automerge.Load(raw)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load workspace")
	}

	meta := WorkspaceMeta{Id: id, SizeBytes: int64(len(raw))}
	if aliasValue, _ := doc.Path("alias").Get(); aliasValue.Kind() == automerge.KindStr {
		meta.Alias = aliasValue.Str()
	}
	if createdAtValue, _ := doc.Path("created_at").Get(); createdAtValue.Kind() == automerge.KindTime {
		meta.CreatedAt = createdAtValue.Time()
	}
	provider := &directoryStorageWorkspace{
		Path: path, Unlocker: unlocker, Logger: d.Logger.With("ws", id),
		Metadata: meta, Doc: &inMemoryWorkspaceProvider{Doc: doc},
	}
	unlocker = nil
	return provider, nil
}

func (d *directoryStorage) ImportWorkspace(ctx context.Context, id string, data []byte) (*WorkspaceMeta, error) {
	if _, err := ulid.Parse(id); err != nil {
		return nil, errors.New("invalid workspace id - expected a valid ulid")
	}
	doc, err := automerge.Load(data)
	if err != nil {
		return nil, errors.New("data is not an automerge document")
	}

	meta := WorkspaceMeta{Id: id, SizeBytes: int64(len(data))}
	if aliasValue, _ := doc.Path("alias").Get(); aliasValue.Kind() != automerge.KindStr {
		return nil, errors.Errorf("automerge document 'alias' is %s, expected %s", aliasValue.Kind(), automerge.KindStr)
	} else {
		meta.Alias = aliasValue.Str()
	}
	if createdAtValue, _ := doc.Path("created_at").Get(); createdAtValue.Kind() != automerge.KindTime {
		return nil, errors.Errorf("automerge document 'created_at' is %s, expected %s", createdAtValue.Kind(), automerge.KindTime)
	} else {
		meta.CreatedAt = createdAtValue.Time()
	}
	if todosValue, _ := doc.Path("todos").Get(); todosValue.Kind() != automerge.KindMap {
		return nil, errors.Errorf("automerge document 'todos' is %s, expected %s", todosValue.Kind(), automerge.KindMap)
	}

	path := filepath.Join(d.Path, id+Suffix)
	tempPath := path + ".temp"
	if err := os.WriteFile(tempPath, doc.Save(), os.FileMode(0644)); err != nil {
		return nil, errors.Wrap(err, "failed to write workspace file")
	}
	if err := os.Rename(tempPath, path); err != nil {
		return nil, errors.Wrap(err, "failed to move workspace file to target")
	}

	return &meta, nil
}

var _ StorageProvider = (*directoryStorage)(nil)

type directoryStorageWorkspace struct {
	Path     string
	Unlocker func()
	Logger   *slog.Logger
	Metadata WorkspaceMeta
	Doc      *inMemoryWorkspaceProvider
}

var _ WorkspaceProvider = (*directoryStorageWorkspace)(nil)
var _ DocProvider = (*directoryStorageWorkspace)(nil)

func (d *directoryStorageWorkspace) Flush() error {
	if d.Unlocker == nil {
		return errors.New("workspace is not locked for writing")
	}
	tempPath := d.Path + ".temp"
	if err := os.WriteFile(tempPath, d.Doc.Doc.Save(), os.FileMode(0600)); err != nil {
		return err
	}
	if err := os.Rename(tempPath, d.Path); err != nil {
		return err
	}
	return nil
}

func (d *directoryStorageWorkspace) Close() error {
	if d.Unlocker == nil {
		return nil
	}
	d.Unlocker()
	d.Unlocker = nil
	return nil
}

func (d *directoryStorageWorkspace) ListTodos(ctx context.Context) ([]Todo, error) {
	return d.Doc.ListTodos(ctx)
}

func (d *directoryStorageWorkspace) GetTodo(ctx context.Context, id string) (*Todo, error) {
	return d.Doc.GetTodo(ctx, id)
}

func (d *directoryStorageWorkspace) CreateTodo(ctx context.Context, params CreateTodoParams) (*Todo, error) {
	return d.Doc.CreateTodo(ctx, params)
}

func (d *directoryStorageWorkspace) EditTodo(ctx context.Context, id string, params EditTodoParams) (*Todo, error) {
	return d.Doc.EditTodo(ctx, id, params)
}

func (d *directoryStorageWorkspace) DeleteTodo(ctx context.Context, id string) error {
	return d.Doc.DeleteTodo(ctx, id)
}

func (d *directoryStorageWorkspace) GetDoc() *automerge.Doc {
	return d.Doc.Doc
}
