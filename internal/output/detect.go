package output

import (
	"os"

	"golang.org/x/term"
)

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func ResolveFormat(format string) string {
	if format == "" || format == "auto" {
		if IsTerminal() {
			return "table"
		}
		return "tsv"
	}
	return format
}
