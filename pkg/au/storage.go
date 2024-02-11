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
	ImportWorkspace(ctx context.Context, id string, data []byte) (*WorkspaceMeta, error)

	GetCurrentWorkspace(ctx context.Context) (string, error)
	SetCurrentWorkspace(ctx context.Context, id string) error
	SetCurrentAuthor(ctx context.Context, author string) error

	OpenWorkspace(ctx context.Context, id string, writeable bool) (WorkspaceProvider, error)
}

type DocProvider interface {
	GetDoc() *automerge.Doc
}

type WorkspaceMeta struct {
	Id            string
	Alias         string
	CreatedAt     time.Time
	SizeBytes     int64
	CurrentAuthor *string
}

type CreateWorkspaceParams struct {
	Alias string
}

type WorkspaceProvider interface {
	Metadata() WorkspaceMeta

	ListTodos(ctx context.Context) ([]Todo, error)
	GetTodo(ctx context.Context, id string) (*Todo, error)
	CreateTodo(ctx context.Context, params CreateTodoParams) (*Todo, error)
	EditTodo(ctx context.Context, id string, params EditTodoParams) (*Todo, error)
	DeleteTodo(ctx context.Context, id string) error

	ListComments(ctx context.Context, todoId string) ([]Comment, error)
	GetComment(ctx context.Context, todoId, commentId string) (*Comment, error)
	CreateComment(ctx context.Context, todoId string, params CreateCommentParams) (*Comment, error)
	EditComment(ctx context.Context, todoId, commentId string, params EditCommentParams) (*Comment, error)
	DeleteComment(ctx context.Context, todoId, commentId string) error

	Flush() error
	Close() error
}

type Todo struct {
	Id           string
	CreatedAt    time.Time
	CreatedBy    string
	UpdatedAt    *time.Time
	UpdatedBy    *string
	CommentCount int

	Title       string
	Description string
	Status      string
	Annotations map[string]string
}

type CreateTodoParams struct {
	Title       string
	Description string
	Status      *string
	Annotations map[string]string
	CreatedBy   string
}

type EditTodoParams struct {
	Title       *string
	Description *string
	Status      *string
	Annotations map[string]string
	UpdatedBy   string
}

type Comment struct {
	Id        string
	CreatedAt time.Time
	CreatedBy string
	UpdatedAt *time.Time
	UpdatedBy *string
	MediaType string
	Content   []byte
}

type CreateCommentParams struct {
	MediaType string
	Content   []byte
	CreatedBy string
}

type EditCommentParams struct {
	Content   []byte
	UpdatedBy string
}
