package cmd

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var pipeToShellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\|\s*(sudo\s+)?(ba)?sh\b`),
	regexp.MustCompile(`(?i)\b(curl|wget)\b.*(&&|;)\s*(sudo\s+)?(ba)?sh\b`),
}

// PlanItem describes one raw command to show before execution.
type PlanItem struct {
	ID, Command string
}

// PipesToShell reports whether a command touches pipe-to-shell (curl|sh pattern),
// worth highlighting to the user (it downloads and runs external code).
func PipesToShell(command string) bool {
	for _, pattern := range pipeToShellPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

// RenderPlan builds a readable plan of what will run: each app id plus the raw
// command, with a marker on pipe-to-shell commands.
func RenderPlan(title string, items []PlanItem) string {
	var b strings.Builder
	fmt.Fprintln(&b, title)
	for _, item := range items {
		marker := " "
		if PipesToShell(item.Command) {
			marker = "⚠"
		}
		fmt.Fprintf(&b, "%s %s\n  %s\n", marker, item.ID, item.Command)
	}
	return b.String()
}

// Confirm reads a yes/no confirmation from in and writes the prompt to out.
// It returns true only for exact "yes" (trimmed, case-insensitive).
func Confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, err
	}

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}

	return strings.EqualFold(strings.TrimSpace(answer), "yes"), nil
}
