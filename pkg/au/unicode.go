package au

import (
	"unicode"
	"unicode/utf8"

	"github.com/pkg/errors"
	"golang.org/x/text/unicode/norm"
)

var ErrContainsInvalidUtf8Runes = errors.New("contains invalid utf8 runes")
var ErrDisallowedCharacter = errors.New("disallowed rune")

var allowedCharacterRanges = append(
	// unicode printable characters
	unicode.PrintRanges,
	// ascii space
	&unicode.RangeTable{
		R16:         []unicode.Range16{{0x0020, 0x0020, 1}},
		LatinOffset: 1,
	},
)

var allowedMultilineCharacterRanges = append(
	allowedCharacterRanges,
	// ascii newline and tab
	&unicode.RangeTable{
		R16:         []unicode.Range16{{0x0009, 0x000A, 1}},
		LatinOffset: 1,
	},
	// ascii carriage return
	&unicode.RangeTable{
		R16:         []unicode.Range16{{0x000D, 0x000D, 1}},
		LatinOffset: 1,
	},
)

func ValidateAndCleanUnicode(input string, allowMultiline bool) (output string, err error) {
	output = input

	if !utf8.ValidString(output) {
		return "", ErrContainsInvalidUtf8Runes
	}
	// normalize to nfc form
	output = norm.NFC.String(output)

	ranges := allowedCharacterRanges
	if allowMultiline {
		ranges = allowedMultilineCharacterRanges
	}

	// forbid characters that aren't in the allowed set
	for ind, r := range output {
		if !unicode.In(r, ranges...) {
			return "", errors.Wrapf(ErrDisallowedCharacter, "position %d", ind)
		}
	}

	return output, nil
}
