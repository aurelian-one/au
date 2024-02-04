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
	var editor string
	if editor = os.Getenv(au.EditorVariable); editor == "" {
		editor = os.Getenv(au.GlobalEditorVariable)
	}
	if editor == "" {
		return title, description, errors.Errorf("cannot use editor, $%s and $%s are not set", au.EditorVariable, au.GlobalEditorVariable)
	}

	tmpf := filepath.Join(os.TempDir(), fmt.Sprintf("au-%x", time.Now().Nanosecond()))
	if err := os.WriteFile(tmpf, []byte(fmt.Sprintf("%s\n\n%s", title, description)), os.FileMode(0600)); err != nil {
		return title, description, errors.Wrap(err, "failed to create temporary file")
	}
	defer os.Remove(tmpf)
	c := exec.CommandContext(ctx, editor, tmpf)
	c.Stdout = os.Stdout
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return title, description, errors.Wrap(err, "failed to execute editor")
	}
	content, err := os.ReadFile(tmpf)
	if err != nil {
		return title, description, errors.Wrap(err, "failed to read temporary file")
	}
	parts := strings.SplitN(string(content), "\n", 2)
	if len(parts) != 2 {
		return title, description, errors.New("edited file was corrupt - do not remove the '-----' line")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
