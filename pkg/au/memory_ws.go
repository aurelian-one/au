package au

import (
	"context"
	"encoding/base64"
	"mime"
	"slices"
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
	if titleValue, _ := item.Map().Get("title"); titleValue.Kind() == automerge.KindText {
		output.Title, _ = titleValue.Text().Get()
	}
	if statusValue, _ := item.Map().Get("status"); statusValue.Kind() == automerge.KindStr {
		output.Status = statusValue.Str()
	}
	if createdAtValue, _ := item.Map().Get("created_at"); createdAtValue.Kind() == automerge.KindTime {
		output.CreatedAt = createdAtValue.Time().In(time.UTC)
	}
	if descriptionValue, _ := item.Map().Get("description"); descriptionValue.Kind() == automerge.KindText {
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

	if commentsValue, _ := item.Map().Get("comments"); commentsValue.Kind() == automerge.KindVoid {
		output.CommentCount = 0
	} else if commentsValue.Kind() == automerge.KindMap {
		output.CommentCount = commentsValue.Map().Len()
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
	if err := newTodo.Set("title", automerge.NewText(params.Title)); err != nil {
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
		for k, v := range params.Annotations {
			if err := ValidateTodoAnnotationKey(k); v != "" && err != nil {
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
		existingTitleValue, _ := todoValue.Map().Get("title")
		if td.Description, err = spliceTextNode(existingTitleValue.Text(), *params.Title); err != nil {
			return nil, err
		}
	}
	if params.Description != nil {
		existingDescriptionValue, _ := todoValue.Map().Get("description")
		if td.Description, err = spliceTextNode(existingDescriptionValue.Text(), *params.Description); err != nil {
			return nil, err
		}
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

func spliceTextNode(node *automerge.Text, newValue string) (string, error) {
	existingStr, _ := node.Get()
	commonPrefix, oldMiddle, newMiddle, _ := stringBreak(existingStr, newValue)
	if err := node.Splice(len(commonPrefix), len(oldMiddle), newMiddle); err != nil {
		return "", errors.Wrap(err, "failed to splice")
	}
	return node.Get()
}

func stringBreak(before, after string) (prefix, oldMiddle, newMiddle, suffix string) {
	beforeRunes, afterRunes := []rune(before), []rune(after)
	prefixEnd := longestCommonPrefix(beforeRunes, afterRunes)
	prefix = string(beforeRunes[:prefixEnd])
	suffixEnd := longestCommonSuffix(beforeRunes[prefixEnd:], afterRunes[prefixEnd:])
	suffix = string(beforeRunes[len(beforeRunes)-suffixEnd:])
	oldMiddle, newMiddle = string(beforeRunes[prefixEnd:len(beforeRunes)-suffixEnd]), string(afterRunes[prefixEnd:len(afterRunes)-suffixEnd])
	return prefix, oldMiddle, newMiddle, suffix
}

func longestCommonPrefix(a, b []rune) (endIndex int) {
	lenA, lenB := len(a), len(b)
	if lenA == 0 || lenB == 0 {
		return 0
	} else if lenA == 1 || lenB == 1 {
		if a[0] == b[0] {
			return 1
		}
		return 0
	}
	maxEnd := min(lenA, lenB)
	mid := max(1, maxEnd/2)
	aHalf, bHalf := a[:mid], b[:mid]
	if slices.Equal(aHalf, bHalf) {
		return mid + longestCommonPrefix(a[mid:], b[mid:])
	} else {
		return longestCommonPrefix(aHalf, bHalf)
	}
}

func longestCommonSuffix(a, b []rune) (endIndex int) {
	lenA, lenB := len(a), len(b)
	if lenA == 0 || lenB == 0 {
		return 0
	} else if lenA == 1 || lenB == 1 {
		if a[0] == b[0] {
			return 1
		}
		return 0
	}
	maxEnd := min(lenA, lenB)
	mid := max(1, maxEnd/2)
	aOffset, bOffset := lenA-mid, lenB-mid
	aHalf, bHalf := a[aOffset:], b[bOffset:]
	if slices.Equal(aHalf, bHalf) {
		return mid + longestCommonSuffix(a[:aOffset], b[:bOffset])
	} else {
		return longestCommonSuffix(aHalf, bHalf)
	}
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

func (p *inMemoryWorkspaceProvider) ListComments(ctx context.Context, todoId string) ([]Comment, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	_, err := getTodoInner(todos, todoId)
	if err != nil {
		return nil, err
	}
	commentsValue, err := todos.Get("comments")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comments in todos")
	} else if commentsValue.Kind() == automerge.KindVoid {
		_ = todos.Set("comments", automerge.NewMap())
		commentsValue, _ = todos.Get("comments")
	} else if commentsValue.Kind() != automerge.KindMap {
		return nil, errors.New("todo comments is not a map")
	}

	commentIds, _ := commentsValue.Map().Keys()
	output := make([]Comment, len(commentIds))
	for i, id := range commentIds {
		c, err := getCommentInner(commentsValue.Map(), id)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get todo")
		}
		output[i] = *c
	}
	return output, nil
}

func getCommentInner(comments *automerge.Map, id string) (*Comment, error) {
	item, err := comments.Get(id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comment")
	} else if item.Kind() != automerge.KindMap {
		return nil, errors.Errorf("comment with id '%s' does not exist", id)
	}
	output := new(Comment)
	output.Id = id
	if mediaTypeValue, _ := item.Map().Get("media_type"); mediaTypeValue.Kind() == automerge.KindStr {
		output.MediaType = mediaTypeValue.Str()
	}
	if contentValue, _ := item.Map().Get("content"); contentValue.Kind() == automerge.KindBytes {
		if output.MediaType == DefaultCommentMediaType {
			output.Content = string(contentValue.Bytes())
		} else {
			output.Content = base64.StdEncoding.EncodeToString(contentValue.Bytes())
		}
	}
	if createdAtValue, _ := item.Map().Get("created_at"); createdAtValue.Kind() == automerge.KindTime {
		output.CreatedAt = createdAtValue.Time().In(time.UTC)
	}
	return output, nil
}

func (p *inMemoryWorkspaceProvider) GetComment(ctx context.Context, todoId, commentId string) (*Comment, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	_, err := getTodoInner(todos, todoId)
	if err != nil {
		return nil, err
	}
	commentsValue, err := todos.Get("comments")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comments in todos")
	} else if commentsValue.Kind() == automerge.KindVoid {
		_ = todos.Set("comments", automerge.NewMap())
		commentsValue, _ = todos.Get("comments")
	} else if commentsValue.Kind() != automerge.KindMap {
		return nil, errors.New("todo comments is not a map")
	}
	return getCommentInner(commentsValue.Map(), commentId)
}

func (p *inMemoryWorkspaceProvider) CreateComment(ctx context.Context, todoId string, params CreateCommentParams) (*Comment, error) {
	if _, _, err := mime.ParseMediaType(params.MediaType); err != nil {
		return nil, errors.Wrap(err, "invalid mime type")
	}

	if params.MediaType == DefaultCommentMediaType {
		if c, err := ValidateAndCleanUnicode(string(params.Content), true); err != nil {
			return nil, err
		} else if len(c) == 0 {
			return nil, errors.New("content is empty")
		} else {
			params.Content = []byte(c)
		}
	} else if len(params.Content) == 0 {
		return nil, errors.New("content is empty")
	}

	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	_, err := getTodoInner(todos, todoId)
	if err != nil {
		return nil, err
	}

	commentsValue, err := todos.Get("comments")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comments in todos")
	} else if commentsValue.Kind() == automerge.KindVoid {
		_ = todos.Set("comments", automerge.NewMap())
		commentsValue, _ = todos.Get("comments")
	} else if commentsValue.Kind() != automerge.KindMap {
		return nil, errors.New("todo comments is not a map")
	}

	newComment := automerge.NewMap()
	newCommentId := ulid.Make().String()
	if err := commentsValue.Map().Set(newCommentId, newComment); err != nil {
		return nil, errors.New("failed to set comment in todo")
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	if err := newComment.Set("created_at", createdAt); err != nil {
		return nil, errors.Wrap(err, "failed to set created_at")
	} else if err := newComment.Set("media_type", params.MediaType); err != nil {
		return nil, errors.Wrap(err, "failed to set media type")
	} else if err := newComment.Set("content", params.Content); err != nil {
		return nil, errors.Wrap(err, "failed to set content")
	}

	if _, err := p.Doc.Commit("created comment " + newCommentId + " in todo " + todoId); err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}
	return getCommentInner(commentsValue.Map(), newCommentId)
}

func (p *inMemoryWorkspaceProvider) EditComment(ctx context.Context, todoId, commentId string, params EditCommentParams) (*Comment, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	_, err := getTodoInner(todos, todoId)
	if err != nil {
		return nil, err
	}

	commentsValue, err := todos.Get("comments")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comments in todos")
	} else if commentsValue.Kind() == automerge.KindVoid {
		_ = todos.Set("comments", automerge.NewMap())
		commentsValue, _ = todos.Get("comments")
	} else if commentsValue.Kind() != automerge.KindMap {
		return nil, errors.New("todo comments is not a map")
	}

	commentValue, err := commentsValue.Map().Get(commentId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comment")
	} else if commentValue.Kind() != automerge.KindMap {
		return nil, errors.Wrap(err, "comment is not a map")
	}

	if mediaTypeValue, err := commentValue.Map().Get("media_type"); err != nil {
		return nil, errors.Wrap(err, "failed to get media type from comment")
	} else if mediaTypeValue.Kind() != automerge.KindStr {
		return nil, errors.Wrap(err, "media type is not a string")
	} else {
		mediaType := mediaTypeValue.Str()
		if mediaType == DefaultCommentMediaType {
			if c, err := ValidateAndCleanUnicode(string(params.Content), true); err != nil {
				return nil, err
			} else if len(c) == 0 {
				return nil, errors.New("content is empty")
			} else {
				params.Content = []byte(c)
			}
		} else if len(params.Content) == 0 {
			return nil, errors.New("content is empty")
		}
	}

	if err = commentValue.Map().Set("content", params.Content); err != nil {
		return nil, errors.Wrap(err, "failed to set content")
	}

	if _, err := p.Doc.Commit("edited comment " + commentId + " in todo " + todoId); err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}
	return getCommentInner(commentsValue.Map(), commentId)
}

func (p *inMemoryWorkspaceProvider) DeleteComment(ctx context.Context, todoId, commentId string) error {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	todos := p.Doc.Path("todos").Map()
	_, err := getTodoInner(todos, todoId)
	if err != nil {
		return err
	}

	commentsValue, err := todos.Get("comments")
	if err != nil {
		return errors.Wrap(err, "failed to get comments in todos")
	} else if commentsValue.Kind() == automerge.KindVoid {
		_ = todos.Set("comments", automerge.NewMap())
		commentsValue, _ = todos.Get("comments")
	} else if commentsValue.Kind() != automerge.KindMap {
		return errors.New("todo comments is not a map")
	} else if err = commentsValue.Map().Delete(commentId); err != nil {
		return errors.New("failed to delete comment")
	}
	if _, err := p.Doc.Commit("deleted comment " + commentId + " in todo " + todoId); err != nil {
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
