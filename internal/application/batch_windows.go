package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

// batchCommand uses one percent-expansion pass on environment-backed quoted
// values with delayed expansion disabled. Embedded quotes and line breaks have
// no qualified representation in this codec and are rejected before creation.
func batchCommand(target string, spec Command) (Command, error) {
	for _, arg := range append([]string{target}, spec.Args...) {
		if strings.ContainsAny(arg, "\x00\r\n\"") {
			return Command{}, fmt.Errorf("batch arguments containing quotes, NUL, or line breaks are not qualified")
		}
	}
	root := ""
	for _, item := range spec.Environment {
		k, v, ok := strings.Cut(item, "=")
		if ok && strings.EqualFold(k, "SystemRoot") {
			root = v
		}
	}
	if !filepath.IsAbs(root) {
		return Command{}, fmt.Errorf("Windows SystemRoot is unavailable for batch handler")
	}
	executable := filepath.Join(root, "System32", "cmd.exe")
	info, err := os.Stat(executable)
	if err != nil || info.IsDir() {
		return Command{}, fmt.Errorf("fixed Windows CMD executable is unavailable")
	}
	env := append([]string{}, spec.Environment...)
	env = append(env, "PORTSCOOP_BATCH_TARGET="+target)
	line := `"` + executable + `" /d /v:off /s /c ""%PORTSCOOP_BATCH_TARGET%"`
	expandedLength := len(utf16.Encode([]rune(executable+target))) + 32
	for i, arg := range spec.Args {
		expandedLength += len(utf16.Encode([]rune(arg))) + 3
		if arg == "" {
			line += ` ""`
			continue
		}
		key := "PORTSCOOP_BATCH_ARG_" + strconv.Itoa(i)
		env = append(env, key+"="+arg)
		line += ` "%` + key + `%"`
	}
	line += `"`
	// CMD's command-line limit is lower than CreateProcess's native limit.
	if len(line) > 8000 || expandedLength > 8000 {
		return Command{}, fmt.Errorf("batch invocation exceeds qualified command-line bound")
	}
	spec.Executable, spec.Args, spec.Environment = executable, nil, env
	spec.WindowsCommandLine = line
	spec.HiddenConsole = spec.Mode == Detach
	return spec, nil
}

func startBatch(ctx context.Context, starter Starter, target string, spec Command) (Process, error) {
	command, err := batchCommand(target, spec)
	if err != nil {
		return nil, err
	}
	return starter.Start(ctx, command)
}
