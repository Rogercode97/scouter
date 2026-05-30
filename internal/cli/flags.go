package cli

import "strings"

// Flags holds parsed global flags.
type Flags struct {
	Verbose      int
	UltraCompact bool
	SkipEnv      bool
	Version      bool
	Help         bool
	Enrich       bool
	Deep         bool
	Env          string
}

// ParseFlags extracts global flags from args and returns remaining args.
// A "--" separator stops flag parsing: everything after it is passed
// verbatim to the underlying command, preventing scouter from consuming
// flags like --help or --version that belong to the proxied tool.
func ParseFlags(args []string) (Flags, []string) {
	var flags Flags
	flags.Env = "production"
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			// Everything after "--" belongs to the underlying command.
			remaining = append(remaining, args[i+1:]...)
			break
		}
		switch {
		case arg == "-vv":
			flags.Verbose = 2
		case arg == "-v":
			if flags.Verbose < 1 {
				flags.Verbose = 1
			}
		case arg == "-u":
			flags.UltraCompact = true
		case arg == "--enrich":
			flags.Enrich = true
		case arg == "--deep":
			flags.Deep = true
		case arg == "--skip-env":
			flags.SkipEnv = true
		case arg == "--version":
			flags.Version = true
		case arg == "--help" || arg == "-h":
			flags.Help = true
		case strings.HasPrefix(arg, "--env="):
			flags.Env = strings.TrimPrefix(arg, "--env=")
		case arg == "--env":
			if i+1 < len(args) {
				flags.Env = args[i+1]
				i++
			}
		case isStackedVerboseFlag(arg):
			flags.Verbose = strings.Count(arg, "v")
		case len(remaining) == 0 && isBuiltInCommand(arg) && i+1 < len(args) && isInfoFlag(args[i+1]):
			remaining = append(remaining, arg)
		default:
			remaining = append(remaining, args[i:]...)
			return flags, remaining
		}
	}

	return flags, remaining
}

// isStackedVerboseFlag detects flags like -vvv, -vvvv (only 'v' chars after dash).
func isStackedVerboseFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	trimmed := strings.TrimLeft(arg, "-")
	return len(trimmed) > 0 && strings.Trim(trimmed, "v") == ""
}

func isBuiltInCommand(arg string) bool {
	switch arg {
	case "init", "gain", "config", "proxy", "ingest", "index", "predict", "mcp", "setup":
		return true
	default:
		return false
	}
}

func isInfoFlag(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "--version"
}
