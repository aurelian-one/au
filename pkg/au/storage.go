package au

import (
	"context"
	"time"

	"github.com/automerge/automerge-go"
)

type StorageProvider interface {
	ListWorkspaces(ctx context.Context) ([]WorkspaceMeta, error)
	GetWorkspace(ctx context.Context, id string) (*WorkspaceMeta, error)
	CreateWorkspace(ctx context.Context, params CreateWorkspaceParams) (*WorkspaceMeta, error)
	DeleteWorkspace(ctx context.Context, id string) error

	GetCurrentWorkspace(ctx context.Context) (string, error)
	SetCurrentWorkspace(ctx context.Context, id string) error

	OpenWorkspace(ctx context.Context, id string, writeable bool) (WorkspaceProvider, error)
}

type DocProvider interface {
	Doc() *automerge.Doc
}

type WorkspaceMeta struct {
	Id        string
	Alias     string
	CreatedAt time.Time
	SizeBytes int64
}

type CreateWorkspaceParams struct {
	Alias string
}

type WorkspaceProvider interface {
	ListTodos(ctx context.Context) ([]Todo, error)
	GetTodo(ctx context.Context, id string) (*Todo, error)
	CreateTodo(ctx context.Context, params CreateTodoParams) (*Todo, error)
	EditTodo(ctx context.Context, id string, params EditTodoParams) (*Todo, error)
	DeleteTodo(ctx context.Context, id string) error
	Flush() error
	Close() error
}

type Todo struct {
	Id        string
	CreatedAt time.Time

	Title       string
	Description string
	Status      string
}

type CreateTodoParams struct {
	Title       string
	Description string
	Status      string
}

type EditTodoParams struct {
	Title       *string
	Description *string
	Status      *string
}
