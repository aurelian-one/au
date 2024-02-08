package au

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/automerge/automerge-go"
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
		Annotations: map[string]string{
			"https://aurelian-one/spec#some-annotation": "something",
		},
		CreatedBy: "Example <email@me.com>",
	})
	assert.NoError(t, err)
	_, err = ulid.Parse(td.Id)
	assert.NoError(t, err)
	assert.WithinRange(t, td.CreatedAt, ws.CreatedAt.Add(-time.Second), time.Now().Add(time.Second))
	assert.Equal(t, "Do the thing", td.Title)
	assert.Equal(t, "Much longer text about doing the thing", td.Description)
	assert.Equal(t, "open", td.Status)
	assert.Equal(t, map[string]string{
		"https://aurelian-one/spec#some-annotation": "something",
	}, td.Annotations)
	if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
		c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
		assert.Equal(t, "created todo "+td.Id, c.Message())
	}

	td2, err := wsp.GetTodo(context.Background(), td.Id)
	assert.NoError(t, err)
	assert.Equal(t, td, td2)
}

func TestCreateTodo_invalid_annotations(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)

	for _, tc := range []struct {
		Key           string
		Value         string
		ExpectedError string
	}{
		{"not a uri", "true", "invalid annotation key 'not a uri': missing a uri scheme"},
		{"about:blank", "true", ""},
		{"http://aurelian.one/thing", "true", "invalid annotation key 'http://aurelian.one/thing': 'aurelian.one' annotations require an https scheme"},
		{"https://user@aurelian.one/thing", "true", "invalid annotation key 'https://user@aurelian.one/thing': 'aurelian.one' annotations cannot have user info"},
		{"https://aurelian.one:44/thing", "true", "invalid annotation key 'https://aurelian.one:44/thing': 'aurelian.one' annotations cannot have a port"},
		{"https://aurelian.one/thing?q=1", "true", "invalid annotation key 'https://aurelian.one/thing?q=1': 'aurelian.one' annotations cannot have a query string"},
		{"https://aurelian.one/thing", "true", "invalid annotation key 'https://aurelian.one/thing': 'aurelian.one' annotation path must match /annotations/* pattern"},
		{"https://aurelian.one/annotations/unknown", "true", "invalid annotation key 'https://aurelian.one/annotations/unknown': 'aurelian.one' 'unknown' annotation is not supported"},
		{"https://aurelian.one/annotations/rank#t", "true", "invalid annotation key 'https://aurelian.one/annotations/rank#t': 'aurelian.one 'rank' annotation cannot have a fragment"},
		{"https://aurelian.one/annotations/label", "true", "invalid annotation key 'https://aurelian.one/annotations/label': 'aurelian.one' 'label' annotation requires a valid fragment"},
		{"https://aurelian/thing", "true", "invalid annotation key 'https://aurelian/thing': 'aurelian' annotation are reserved"},
		{"https://aurelian.one/annotations/label#foo", "true", ""},
	} {
		t.Run(tc.Key+"="+tc.Value, func(t *testing.T) {
			_, err := wsp.CreateTodo(context.Background(), CreateTodoParams{
				Title:       "Do the thing",
				Description: "Much longer text about doing the thing",
				Annotations: map[string]string{
					tc.Key: tc.Value,
				},
				CreatedBy: "Example <email@me.com>",
			})
			if tc.ExpectedError != "" {
				assert.EqualError(t, err, tc.ExpectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEditTodo_success(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)
	td, err := wsp.CreateTodo(context.Background(), CreateTodoParams{
		Title:       "Do the thing",
		Description: "Much longer text about doing the thing",
		CreatedBy:   "Example <email@me.com>",
	})
	assert.NoError(t, err)
	newTitle, newDescription, newStatus := "Do the other thing", "Short description", "closed"
	td2, err := wsp.EditTodo(context.Background(), td.Id, EditTodoParams{
		Title:       &newTitle,
		Description: &newDescription,
		Status:      &newStatus,
		UpdatedBy:   "Example <email@me.com>",
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

func TestEditTodo_check_efficient_description(t *testing.T) {
	s := newDirectoryStorage(t)
	ws, _ := s.CreateWorkspace(context.Background(), CreateWorkspaceParams{Alias: "testing"})
	wsp, _ := s.OpenWorkspace(context.Background(), ws.Id, true)
	td, err := wsp.CreateTodo(context.Background(), CreateTodoParams{
		Title:       "Do the thing",
		Description: "my original text.",
		CreatedBy:   "Example <email@me.com>",
	})
	assert.NoError(t, err)

	t.Run("empty update", func(t *testing.T) {
		_, err = wsp.EditTodo(context.Background(), td.Id, EditTodoParams{
			UpdatedBy: "Example <email@me.com>",
		})
		assert.NoError(t, err)
		if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
			c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
			assert.Equal(t, "edited todo "+td.Id, c.Message())
			assert.Len(t, automerge.SaveChanges([]*automerge.Change{c}), 190)
		}
	})

	t.Run("changing one byte", func(t *testing.T) {
		newDescription := "my original text!"
		_, err = wsp.EditTodo(context.Background(), td.Id, EditTodoParams{
			Description: &newDescription,
			UpdatedBy:   "Example <email@me.com>",
		})
		assert.NoError(t, err)
		if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
			c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
			assert.Equal(t, "edited todo "+td.Id, c.Message())
			assert.Len(t, automerge.SaveChanges([]*automerge.Change{c}), 157)
		}
	})

	t.Run("changing all bytes", func(t *testing.T) {
		newDescription := "MY ORIGINAL TEXT."
		_, err = wsp.EditTodo(context.Background(), td.Id, EditTodoParams{
			Description: &newDescription,
			UpdatedBy:   "Example <email@me.com>",
		})
		assert.NoError(t, err)
		if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
			c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
			assert.Equal(t, "edited todo "+td.Id, c.Message())
			assert.Len(t, automerge.SaveChanges([]*automerge.Change{c}), 191)
		}
	})
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
		CreatedBy:   "Example <email@me.com>",
	})
	assert.NoError(t, err)
	assert.NoError(t, wsp.DeleteTodo(context.Background(), td.Id))
	if h := wsp.(DocProvider).GetDoc().Heads(); assert.Len(t, h, 1) {
		c, _ := wsp.(DocProvider).GetDoc().Change(h[0])
		assert.Equal(t, "deleted todo "+td.Id, c.Message())
	}
}

func TestStringBreak(t *testing.T) {
	for _, tc := range []string{
		"     ",
		"a a a   ",
		"ab ac a b c ",
		"abc adc a b d c",
		"ab abc ab  c ",
		"one-two-three one-zero-three one- tw zer o-three",
		"something   something  ",
		" something   something ",
		"one two  one two ",
	} {
		t.Run(tc, func(t *testing.T) {
			parts := strings.Split(tc, " ")
			a, b, c, d := stringBreak(parts[0], parts[1])
			assert.Equal(t, []string{parts[2], parts[3], parts[4], parts[5]}, []string{a, b, c, d})
		})
	}
}
