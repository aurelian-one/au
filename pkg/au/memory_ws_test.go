package au

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestListTodos_empty(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, false)
	todos, err := wsp.ListTodos(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, todos)
}

func TestGetTodo_missing(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, false)
	_, err := wsp.GetTodo(context.Background(), "thing")
	assert.EqualError(t, err, "failed to get todo: todo with id 'thing' does not exist")
}

func TestCreateTodo_success(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)
	td, err := wsp.CreateTodo(context.Background(), CreateTodoParams{
		Title:       "Do the thing",
		Description: "Much longer text about doing the thing",
		Status:      "open",
	})
	assert.NoError(t, err)
	_, err = ulid.Parse(td.Id)
	assert.NoError(t, err)
	assert.WithinRange(t, td.CreatedAt, ws.CreatedAt.Add(-time.Second), time.Now().Add(time.Second))
	assert.Equal(t, "Do the thing", td.Title)
	assert.Equal(t, "Much longer text about doing the thing", td.Description)
	assert.Equal(t, "open", td.Status)
	if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
		c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
		assert.Equal(t, "created todo "+td.Id, c.Message())
	}

	td2, err := wsp.GetTodo(context.Background(), td.Id)
	assert.NoError(t, err)
	assert.Equal(t, td, td2)
}

func TestEditTodo_success(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)
	td, err := wsp.CreateTodo(context.Background(), CreateTodoParams{
		Title:       "Do the thing",
		Description: "Much longer text about doing the thing",
		Status:      "open",
	})
	assert.NoError(t, err)
	newTitle, newDescription, newStatus := "Do the other thing", "Short description", "closed"
	td2, err := wsp.EditTodo(context.Background(), td.Id, EditTodoParams{
		Title:       &newTitle,
		Description: &newDescription,
		Status:      &newStatus,
	})
	assert.NoError(t, err)
	if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
		c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
		assert.Equal(t, "edited todo "+td.Id, c.Message())
	}

	td3, err := wsp.GetTodo(context.Background(), td.Id)
	assert.NoError(t, err)
	assert.Equal(t, "Do the other thing", td3.Title)
	assert.Equal(t, "Short description", td3.Description)
	assert.Equal(t, "closed", td3.Status)

	assert.NotEqual(t, td, td2)
	assert.Equal(t, td2, td3)
}

func TestDeleteTodo_missing(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)
	assert.EqualError(t, wsp.DeleteTodo(context.Background(), "something"), "todo with id 'something' does not exist")
}

func TestDeleteTodo_success(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)
	td, err := wsp.CreateTodo(context.Background(), CreateTodoParams{
		Title:       "Do the thing",
		Description: "Much longer text about doing the thing",
		Status:      "open",
	})
	assert.NoError(t, err)
	assert.NoError(t, wsp.DeleteTodo(context.Background(), td.Id))
	if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
		c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
		assert.Equal(t, "deleted todo "+td.Id, c.Message())
	}
}
