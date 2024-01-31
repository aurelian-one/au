package au

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

const MinimumTodoTitleLength = 3
const MaximumTodoTitleLength = 200
const MaximumDescriptionLength = 5000

func ValidateWorkspaceAlias(input string) (string, error) {
	if pa, err := ValidateAndCleanUnicode(input, false); err != nil {
		return "", errors.Wrap(err, "invalid alias")
	} else if pa, d := strings.TrimSpace(pa), MinimumAliasLength; len(pa) < d {
		return "", errors.Errorf("alias is too short, it should be at least %d characters", d)
	} else if d := MaximumAliasLength; len(pa) > d {
		return "", errors.Errorf("alias is too long, it should be at most %d characters", d)
	} else {
		return pa, nil
	}
}

func ValidateTodoTitle(input string) (string, error) {
	if pt, err := ValidateAndCleanUnicode(input, false); err != nil {
		return "", errors.Wrap(err, "invalid title")
	} else if pt, d := strings.TrimSpace(pt), MinimumTodoTitleLength; len(pt) < d {
		return "", errors.Errorf("title is too short, it should be at least %d characters", d)
	} else if d := MaximumTodoTitleLength; len(pt) > d {
		return "", errors.Errorf("title is too long, it should be at most %d characters", d)
	} else {
		return pt, nil
	}
}

func ValidateTodoDescription(input string) (string, error) {
	if pt, err := ValidateAndCleanUnicode(input, true); err != nil {
		return "", errors.Wrap(err, "invalid description")
	} else if d := MaximumDescriptionLength; len(pt) > d {
		return "", errors.Errorf("description is too long, it should be at most %d characters", d)
	} else {
		return pt, nil
	}
}

func ValidateTodoStatus(input string) (string, error) {
	switch input {
	case "open":
	case "closed":
	default:
		return "", errors.New("status must be open or closed")
	}
	return input, nil
}

func ValidateTodoAnnotationKey(key string) error {
	if len(key) > 255 {
		return errors.New("uri is too long")
	}
	u, err := url.Parse(key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(u.Scheme) == "" {
		return errors.New("missing a uri scheme")
	}
	return nil
}
