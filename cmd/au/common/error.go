package common

import (
	"fmt"
)

type ExitWithCode struct {
	Code int
}

func (e *ExitWithCode) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}
