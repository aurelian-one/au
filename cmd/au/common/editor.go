package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/aurelian-one/au/pkg/au"
)

func EditTitleAndDescription(ctx context.Context, title, description string) (string, string, error) {
	content, err := EditContent(ctx, fmt.Sprintf("%s\n\n%s", title, description))
	if err != nil {
		return title, description, err
	}
	parts := strings.SplitN(content, "\n", 2)
	if len(parts) != 2 {
		return title, description, errors.New("edited file was corrupt - do not remove the '-----' line")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func EditContent(ctx context.Context, input string) (string, error) {
	var editor string
	if editor = os.Getenv(au.EditorVariable); editor == "" {
		editor = os.Getenv(au.GlobalEditorVariable)
	}
	if editor == "" {
		return input, errors.Errorf("cannot use editor, $%s and $%s are not set", au.EditorVariable, au.GlobalEditorVariable)
	}

	tmpf := filepath.Join(os.TempDir(), fmt.Sprintf("au-%x", time.Now().Nanosecond()))
	if err := os.WriteFile(tmpf, []byte(input), os.FileMode(0600)); err != nil {
		return input, errors.Wrap(err, "failed to create temporary file")
	}
	defer os.Remove(tmpf)
	c := exec.CommandContext(ctx, editor, tmpf)
	c.Stdout = os.Stdout
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return input, errors.Wrap(err, "failed to execute editor")
	}
	content, err := os.ReadFile(tmpf)
	if err != nil {
		return input, errors.Wrap(err, "failed to read temporary file")
	}
	return string(content), nil
}
