package au

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
)

func Confirm(question string, stdout *os.File, stdin *os.File) (bool, error) {
	if !isatty.IsTerminal(stdin.Fd()) && !isatty.IsCygwinTerminal(stdin.Fd()) {
		return false, errors.New("standard input is not a tty - provide --yes to confirm")
	}
	question = strings.TrimSpace(question) + " [y/n]:\n"
	reader := bufio.NewReader(stdin)
	for {
		_, _ = fmt.Fprintf(stdout, question)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, errors.New("failed to read line")
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true, nil
		} else if response == "n" || response == "no" {
			return false, nil
		}
	}
}
