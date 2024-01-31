package au

import (
	"context"
	"strings"
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
	} else if item.Kind() != automerge.KindMap {
		return nil, errors.Errorf("todo with id '%s' does not exist", id)
	}
	output := new(Todo)
	output.Id = id
	if titleValue, _ := item.Map().Get("title"); titleValue.Kind() == automerge.KindStr {
		output.Title = titleValue.Str()
	}
	if statusValue, _ := item.Map().Get("status"); statusValue.Kind() == automerge.KindStr {
		output.Status = statusValue.Str()
	}
	if createdAtValue, _ := item.Map().Get("created_at"); createdAtValue.Kind() == automerge.KindTime {
		output.CreatedAt = createdAtValue.Time().In(time.UTC)
	}
	descriptionValue, _ := item.Map().Get("description")
	switch descriptionValue.Kind() {
	case automerge.KindStr:
		output.Description = descriptionValue.Str()
	case automerge.KindText:
		output.Description, _ = descriptionValue.Text().Get()
	}

	output.Annotations = make(map[string]string)
	if annotationsValue, _ := item.Map().Get("annotations"); annotationsValue.Kind() == automerge.KindMap {
		keys, _ := annotationsValue.Map().Keys()
		for _, key := range keys {
			if value, _ := annotationsValue.Map().Get(key); value.Kind() == automerge.KindStr {
				output.Annotations[key] = value.Str()
			}
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
	var err error
	params.Title, err = ValidateTodoTitle(params.Title)
	if err != nil {
		return nil, err
	}
	params.Description, err = ValidateTodoDescription(params.Description)
	if err != nil {
		return nil, err
	}

	status := "open"
	if params.Status != nil {
		status, err = ValidateTodoStatus(*params.Status)
		if err != nil {
			return nil, err
		}
	}

	if params.Annotations != nil {
		for k, v := range params.Annotations {
			if err := ValidateTodoAnnotationKey(k); err != nil {
				return nil, errors.Wrapf(err, "invalid annotation key '%s'", k)
			} else if v == "" {
				return nil, errors.Errorf("annotation '%s' has an empty value", k)
			}
		}
	} else {
		params.Annotations = make(map[string]string)
	}

	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	todoId := ulid.Make().String()
	// TODO: check for conflict

	newTodo := automerge.NewMap()
	if err := todos.Set(todoId, newTodo); err != nil {
		return nil, errors.Wrap(err, "failed to set todo entry")
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if err := newTodo.Set("status", status); err != nil {
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

	newAnnotations := automerge.NewMap()
	_ = newTodo.Set("annotations", newAnnotations)
	for k, v := range params.Annotations {
		_ = newAnnotations.Set(k, v)
	}

	if _, err := p.Doc.Commit("created todo " + todoId); err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}
	return getTodoInner(todos, todoId)
}

func (p *inMemoryWorkspaceProvider) EditTodo(ctx context.Context, id string, params EditTodoParams) (*Todo, error) {
	if params.Title != nil {
		o, err := ValidateTodoTitle(*params.Title)
		if err != nil {
			return nil, err
		}
		params.Title = &o
	}
	if params.Description != nil {
		o, err := ValidateTodoDescription(*params.Description)
		if err != nil {
			return nil, err
		}
		params.Description = &o
	}
	if params.Status != nil {
		o, err := ValidateTodoStatus(*params.Status)
		if err != nil {
			return nil, err
		}
		params.Status = &o
	}

	if params.Title != nil {
		if pt, err := ValidateAndCleanUnicode(*params.Title, false); err != nil {
			return nil, errors.Wrap(err, "invalid title")
		} else if pt, d := strings.TrimSpace(pt), MinimumTodoTitleLength; len(pt) < d {
			return nil, errors.Errorf("title is too short, it should be at least %d characters", d)
		} else if d := MaximumTodoTitleLength; len(pt) > d {
			return nil, errors.Errorf("title is too long, it should be at most %d characters", d)
		} else {
			params.Title = &pt
		}
	}

	if params.Description != nil {
		if pt, err := ValidateAndCleanUnicode(*params.Description, true); err != nil {
			return nil, errors.Wrap(err, "invalid description")
		} else if d := MaximumDescriptionLength; len(pt) > d {
			return nil, errors.Errorf("description is too long, it should be at most %d characters", d)
		} else {
			params.Description = &pt
		}
	}

	if params.Annotations != nil {
		for k := range params.Annotations {
			if err := ValidateTodoAnnotationKey(k); err != nil {
				return nil, errors.Wrapf(err, "invalid annotation key '%s'", k)
			}
		}
	} else {
		params.Annotations = make(map[string]string)
	}

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

	annotationsValue, _ := todoValue.Map().Get("annotations")
	if annotationsValue.Kind() == automerge.KindVoid {
		annotationsMap := automerge.NewMap()
		_ = todoValue.Map().Set("annotations", annotationsMap)
		annotationsValue, _ = todoValue.Map().Get("annotations")
	}
	for k, v := range params.Annotations {
		if v == "" {
			_ = annotationsValue.Map().Delete(k)
		} else {
			_ = annotationsValue.Map().Set(k, v)
		}
	}

	if _, err := p.Doc.Commit("edited todo " + id); err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}
	return getTodoInner(todos, id)
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

func (p *inMemoryWorkspaceProvider) GetDoc() *automerge.Doc {
	return p.Doc
}

var _ WorkspaceProvider = (*inMemoryWorkspaceProvider)(nil)
var _ DocProvider = (*inMemoryWorkspaceProvider)(nil)
