package au

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const MinimumTodoTitleLength = 3
const MaximumTodoTitleLength = 200
const MaximumDescriptionLength = 5000
const DefaultCommentMediaType = "text/markdown"

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
	if u.Hostname() == ReservedAnnotationHostname {
		// we control this schema and there are only particular valid values here
		if u.Scheme != "https" {
			return errors.Errorf("'%s' annotations require an https scheme", u.Hostname())
		} else if u.User != nil {
			return errors.Errorf("'%s' annotations cannot have user info", u.Hostname())
		} else if u.Port() != "" {
			return errors.Errorf("'%s' annotations cannot have a port", u.Hostname())
		} else if u.RawQuery != "" {
			return errors.Errorf("'%s' annotations cannot have a query string", u.Hostname())
		}
		parts := strings.Split(u.Path, "/")
		if len(parts) != 3 || parts[1] != "annotations" || parts[2] == "" {
			return errors.Errorf("'%s' annotation path must match /annotations/* pattern", u.Hostname())
		}

		// extra validation for known keys
		switch parts[2] {
		case "label":
			if u.Fragment == "" {
				return errors.Errorf("'%s' '%s' annotation requires a valid fragment", u.Hostname(), parts[2])
			}
		case "rank":
			if u.RawFragment != "" || u.Fragment != "" {
				return errors.Errorf("'%s '%s' annotation cannot have a fragment", u.Hostname(), parts[2])
			}
		default:
			return errors.Errorf("'%s' '%s' annotation is not supported", u.Hostname(), parts[2])
		}

	} else if u.Hostname() == ReservedAnnotationShortHostname {
		return errors.Errorf("'%s' annotation are reserved", u.Hostname())
	}
	return nil
}

var validAuthorPattern = regexp.MustCompile(`^\S+( \S+)* <\S+@\S+>$`)

func ValidatedAuthor(input string) error {
	if !validAuthorPattern.MatchString(input) {
		return errors.New("invalid author string, expected 'Name <email>'")
	}
	return nil
}
