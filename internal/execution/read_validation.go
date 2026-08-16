package execution

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxGrepPatternBytes           = 1024
	piCompatibleRemoteResultLimit = 12 << 10
)

func validateGrepPattern(value string) (*regexp.Regexp, error) {
	if len(value) == 0 || len(value) > maxGrepPatternBytes ||
		strings.TrimSpace(value) != value || !utf8.ValidString(value) ||
		executionTextHasUnsafeControls([]byte(value)) {
		return nil, ErrInvalidExecutionInput
	}
	pattern, err := regexp.Compile(value)
	if err != nil {
		return nil, ErrInvalidExecutionInput
	}
	return pattern, nil
}

func executionTextHasUnsafeControls(content []byte) bool {
	for _, value := range string(content) {
		if unicode.IsControl(value) && value != '\n' && value != '\r' && value != '\t' {
			return true
		}
	}
	return false
}
