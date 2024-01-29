package au

import (
	"context"
	"sync"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
)

type inMemoryWorkspaceProvider struct {
	Doc  *automerge.Doc
	Lock sync.Mutex
}

func (p *inMemoryWorkspaceProvider) ListTodos(ctx context.Context) ([]Todo, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	todos := p.Doc.Path("todos").Map()
	todoIds, _ := todos.Keys()
	output := make([]Todo, len(todoIds))
	for i, id := range todoIds {
		td, err := getTodoInner(todos, id)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get todo")
		}
		output[i] = *td
	}
	return output, nil
}

func getTodoInner(todos *automerge.Map, id string) (*Todo, error) {
	item, err := todos.Get(id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get todo")
	}
	output := new(Todo)
	output.Id = id
	if titleValue, _ := item.Map().Get("title"); titleValue != nil {
		output.Title = titleValue.Str()
	}
	if statusValue, _ := item.Map().Get("status"); statusValue != nil {
		output.Status = statusValue.Str()
	}
	if createdAtValue, _ := item.Map().Get("created_at"); createdAtValue != nil {
		output.CreatedAt = createdAtValue.Time()
	}
	if descriptionValue, _ := item.Map().Get("description"); descriptionValue != nil {
		if asText, _ := descriptionValue.Text().Get(); err == nil {
			output.Description = asText
		} else {
			output.Description = descriptionValue.Str()
		}
	}
	return output, nil
}

func (p *inMemoryWorkspaceProvider) GetTodo(ctx context.Context, id string) (*Todo, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	todos := p.Doc.Path("todos").Map()
	td, err := getTodoInner(todos, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get todo")
	}
	return td, nil
}

func (p *inMemoryWorkspaceProvider) CreateTodo(ctx context.Context, params CreateTodoParams) (*Todo, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	todoId := ulid.Make().String()
	// TODO: check for conflict
	newTodo := automerge.NewMap()
	if err := todos.Set(todoId, newTodo); err != nil {
		return nil, errors.Wrap(err, "failed to set todo entry")
	}

	createdAt := time.Now()
	if err := newTodo.Set("status", params.Status); err != nil {
		return nil, errors.Wrap(err, "failed to set status")
	} else if err := newTodo.Set("created_at", createdAt); err != nil {
		return nil, errors.Wrap(err, "failed to set created_at")
	}
	if err := newTodo.Set("title", params.Title); err != nil {
		return nil, errors.Wrap(err, "failed to set title")
	}
	if err := newTodo.Set("description", automerge.NewText(params.Description)); err != nil {
		return nil, errors.Wrap(err, "failed to set description")
	}
	if _, err := p.Doc.Commit("added todo " + todoId); err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}
	return &Todo{Id: todoId, CreatedAt: createdAt, Status: params.Status, Title: params.Title, Description: params.Description}, nil
}

func (p *inMemoryWorkspaceProvider) EditTodo(ctx context.Context, id string, params EditTodoParams) (*Todo, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	td, err := getTodoInner(todos, id)
	if err != nil {
		return nil, err
	}
	todoValue, _ := p.Doc.Path("todos").Map().Get(id)
	if params.Title != nil {
		if err := todoValue.Map().Set("title", *params.Title); err != nil {
			return nil, errors.Wrap(err, "failed to set title on existing todo")
		}
		td.Title = *params.Title
	}
	if params.Description != nil {
		if err := todoValue.Map().Set("description", *params.Description); err != nil {
			return nil, errors.Wrap(err, "failed to set description on existing todo")
		}
		td.Description = *params.Description
	}
	if params.Status != nil {
		if err := todoValue.Map().Set("status", *params.Status); err != nil {
			return nil, errors.Wrap(err, "failed to set status on existing todo")
		}
		td.Status = *params.Status
	}
	if _, err := p.Doc.Commit("edited todo " + id); err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}
	return td, nil
}

func (p *inMemoryWorkspaceProvider) DeleteTodo(ctx context.Context, id string) error {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	_, err := getTodoInner(todos, id)
	if err != nil {
		return err
	}
	if err := todos.Delete(id); err != nil {
		return err
	}
	if _, err := p.Doc.Commit("deleted todo " + id); err != nil {
		return errors.Wrap(err, "failed to commit")
	}
	return nil
}

func (p *inMemoryWorkspaceProvider) Flush() error {
	return nil
}

func (p *inMemoryWorkspaceProvider) Close() error {
	return nil
}

var _ WorkspaceProvider = (*inMemoryWorkspaceProvider)(nil)
