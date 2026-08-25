package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/wminjay/codex-sessions/internal/session"
)

const (
	ProviderOriginal = "original"
	ProviderOpenAI   = "openai"
	ProviderCustom   = "custom"
	ActionResume     = "resume"
	ActionFork       = "fork"
)

type Request struct {
	Session  session.Session
	Action   string
	Provider string
}

type Options struct {
	CodexBin      string
	CustomProfile string
	Yolo          bool
}

type Command struct {
	Executable string
	Args       []string
	Env        []string
	Session    session.Session
}

func Build(request Request, options Options) (Command, error) {
	if request.Action != ActionResume && request.Action != ActionFork {
		return Command{}, fmt.Errorf("unsupported action: %s", request.Action)
	}
	provider := request.Provider
	if provider == "" || provider == ProviderOriginal {
		provider = request.Session.Provider
	}
	if options.CodexBin == "" {
		options.CodexBin = "codex"
	}
	args := []string{request.Action, request.Session.ID}
	if provider == ProviderCustom && options.CustomProfile != "" {
		args = append(args, "-p", options.CustomProfile)
	}
	if provider != "" && provider != "unknown" {
		args = append(args, "-c", "model_provider="+strconv.Quote(provider))
	}
	if options.Yolo {
		// The public switch is -yolo; Codex currently recognizes this alias.
		args = append(args, "--yolo")
	}
	environment := replaceEnvironment(os.Environ(), map[string]string{
		"CODEX_HOME":        request.Session.CodexHome,
		"CODEX_SQLITE_HOME": request.Session.SQLiteHome,
	})
	return Command{
		Executable: options.CodexBin,
		Args:       args,
		Env:        environment,
		Session:    request.Session,
	}, nil
}

func Run(command Command) error {
	if _, err := exec.LookPath(command.Executable); err != nil {
		return fmt.Errorf("find Codex executable %q: %w", command.Executable, err)
	}
	process := exec.Command(command.Executable, command.Args...)
	process.Env = command.Env
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	return process.Run()
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 1
}

func Printable(command Command) string {
	environment := []string{
		"CODEX_HOME=" + shellQuote(command.Session.CodexHome),
		"CODEX_SQLITE_HOME=" + shellQuote(command.Session.SQLiteHome),
	}
	parts := []string{shellQuote(command.Executable)}
	for _, argument := range command.Args {
		parts = append(parts, shellQuote(argument))
	}
	if runtime.GOOS == "windows" {
		return "set " + strings.Join(environment, " && set ") + " && " + strings.Join(parts, " ")
	}
	return strings.Join(append(environment, parts...), " ")
}

func replaceEnvironment(environment []string, replacements map[string]string) []string {
	result := make([]string, 0, len(environment)+len(replacements))
	seen := make(map[string]bool)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if replacement, replace := replacements[name]; replace {
			if !seen[name] {
				result = append(result, name+"="+replacement)
				seen[name] = true
			}
			continue
		}
		result = append(result, entry)
	}
	for name, replacement := range replacements {
		if !seen[name] {
			result = append(result, name+"="+replacement)
		}
	}
	return result
}

func shellQuote(value string) string {
	if runtime.GOOS == "windows" {
		if value == "" || strings.ContainsAny(value, " \t\"") {
			return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
		}
		return value
	}
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!&|;()<>*?[]{}") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
