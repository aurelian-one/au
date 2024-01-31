package au

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAndCleanUnicode(t *testing.T) {
	for _, tc := range []struct {
		Name           string
		Input          string
		Multiline      bool
		ExpectedOutput string
		ExpectedError  string
	}{
		{Name: "normal ascii", Input: "abcdefg 0123456789 stuff"},
		{Name: "forbid control character", Input: "some\000thing", ExpectedError: "position 4: disallowed rune"},
		{Name: "no newlines if not allowed", Input: "some\nthing", ExpectedError: "position 4: disallowed rune"},
		{Name: "allow newlines if allowed", Input: "some\nthing", Multiline: true},
		{Name: "unicode allowed", Input: "👍"},
		{Name: "markdown allowed", Input: `# stuff

<img src=""/>

| thing |
|-------|

`, Multiline: true},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			o, e := ValidateAndCleanUnicode(tc.Input, tc.Multiline)
			if tc.ExpectedError != "" {
				assert.EqualError(t, e, tc.ExpectedError)
			} else {
				assert.NoError(t, e)
				if tc.ExpectedOutput == "" {
					assert.Equal(t, tc.Input, o)
				} else {
					assert.Equal(t, tc.ExpectedOutput, o)
				}
			}
		})
	}
}
