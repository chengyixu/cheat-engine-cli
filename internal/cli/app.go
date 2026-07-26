package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	Version   = "0.1.0-dev"
	Commit    = "none"
	BuildDate = "unknown"
)

const defaultEndpoint = "127.0.0.1:52736"

type globalOptions struct {
	endpoint       string
	connectionName string
	timeout        time.Duration
	human          bool
	pretty         bool
	quiet          bool
	fields         []string
	dryRun         bool
	help           bool
	version        bool
	brief          bool
}

type app struct {
	context context.Context
	stdout  io.Writer
	stderr  io.Writer
	options globalOptions
}

func Run(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	options, remainingArguments, err := parseGlobalOptions(arguments)
	application := &app{context: ctx, stdout: stdout, stderr: stderr, options: options}
	if err != nil {
		return application.writeError("", err)
	}
	command := commandName(remainingArguments)
	startedAt := time.Now()
	result, err := application.execute(remainingArguments)
	if err != nil {
		return application.writeError(command, err)
	}
	if err := application.writeSuccess(command, result, startedAt); err != nil {
		return application.writeError(command, err)
	}
	return 0
}

func (application *app) execute(arguments []string) (commandResult, error) {
	if application.options.version {
		return application.versionResult(), nil
	}
	if application.options.brief {
		return application.briefResult(), nil
	}
	if application.options.help {
		return application.helpResult(helpTopic(arguments)), nil
	}
	if len(arguments) == 0 {
		return application.helpResult(""), nil
	}

	switch arguments[0] {
	case "help":
		topic := ""
		if len(arguments) > 1 {
			topic = strings.Join(arguments[1:], " ")
		}
		return application.helpResult(topic), nil
	case "version":
		return application.versionResult(), nil
	case "brief":
		return application.briefResult(), nil
	case "server":
		return application.executeServer(arguments[1:])
	case "process":
		return application.executeProcess(arguments[1:])
	case "module":
		return application.executeModule(arguments[1:])
	case "thread":
		return application.executeThread(arguments[1:])
	case "debug":
		return application.executeDebug(arguments[1:])
	case "memory":
		return application.executeMemory(arguments[1:])
	case "remote":
		return application.executeRemote(arguments[1:])
	case "pipe":
		return application.executePipe(arguments[1:])
	case "symbol":
		return application.executeSymbol(arguments[1:])
	case "skills":
		return application.executeSkills(arguments[1:])
	case "issue":
		return application.executeIssue(arguments[1:])
	case "completion":
		return application.executeCompletion(arguments[1:])
	case "self-check":
		return application.executeSelfCheck(arguments[1:])
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown command %q", arguments[0]), "Run 'cecli help' to list supported commands.")
	}
}

func parseGlobalOptions(arguments []string) (globalOptions, []string, error) {
	options := globalOptions{
		endpoint:       envOrDefault("CECLI_ENDPOINT", defaultEndpoint),
		connectionName: os.Getenv("CECLI_CONNECTION_NAME"),
		timeout:        30 * time.Second,
	}
	if strings.EqualFold(os.Getenv("CECLI_OUTPUT"), "human") {
		options.human = true
	}
	if configuredTimeout := os.Getenv("CECLI_TIMEOUT"); configuredTimeout != "" {
		parsedTimeout, err := time.ParseDuration(configuredTimeout)
		if err != nil {
			return options, nil, usageError("invalid CECLI_TIMEOUT", "Use a duration such as 30s or 2m.")
		}
		options.timeout = parsedTimeout
	}
	if configuredFields := os.Getenv("CECLI_FIELDS"); configuredFields != "" {
		fields, err := parseFieldSelection(configuredFields)
		if err != nil {
			return options, nil, err
		}
		options.fields = fields
	}

	remaining := make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch {
		case argument == "--human":
			options.human = true
		case argument == "--agent":
			options.human = false
		case argument == "--pretty":
			options.pretty = true
		case argument == "--quiet" || argument == "-q":
			options.quiet = true
		case argument == "--dry-run":
			options.dryRun = true
		case argument == "--help" || argument == "-h":
			options.help = true
		case argument == "--version" || argument == "-v":
			options.version = true
		case argument == "--brief":
			options.brief = true
		case strings.HasPrefix(argument, "--fields="):
			fields, err := parseFieldSelection(strings.TrimPrefix(argument, "--fields="))
			if err != nil {
				return options, nil, err
			}
			options.fields = append(options.fields, fields...)
		case argument == "--fields":
			index++
			if index >= len(arguments) {
				return options, nil, missingRequired("--fields value", "Use --fields pid,name or --fields process.architecture.")
			}
			fields, err := parseFieldSelection(arguments[index])
			if err != nil {
				return options, nil, err
			}
			options.fields = append(options.fields, fields...)
		case strings.HasPrefix(argument, "--endpoint="):
			options.endpoint = strings.TrimPrefix(argument, "--endpoint=")
		case argument == "--endpoint":
			index++
			if index >= len(arguments) {
				return options, nil, missingRequired("--endpoint value", "Use --endpoint 127.0.0.1:52736.")
			}
			options.endpoint = arguments[index]
		case strings.HasPrefix(argument, "--connection-name="):
			options.connectionName = strings.TrimPrefix(argument, "--connection-name=")
		case argument == "--connection-name":
			index++
			if index >= len(arguments) {
				return options, nil, missingRequired("--connection-name value", "Use a diagnostic name such as --connection-name ci-worker-1.")
			}
			options.connectionName = arguments[index]
		case strings.HasPrefix(argument, "--timeout="):
			parsedTimeout, err := time.ParseDuration(strings.TrimPrefix(argument, "--timeout="))
			if err != nil {
				return options, nil, usageError("invalid --timeout value", "Use a duration such as --timeout 30s or --timeout 2m.")
			}
			options.timeout = parsedTimeout
		case argument == "--timeout":
			index++
			if index >= len(arguments) {
				return options, nil, missingRequired("--timeout value", "Use --timeout 30s.")
			}
			parsedTimeout, err := time.ParseDuration(arguments[index])
			if err != nil {
				return options, nil, usageError("invalid --timeout value", "Use a duration such as --timeout 30s or --timeout 2m.")
			}
			options.timeout = parsedTimeout
		default:
			remaining = append(remaining, argument)
		}
	}
	if strings.TrimSpace(options.endpoint) == "" {
		return options, nil, usageError("endpoint cannot be empty", "Use --endpoint host:port.")
	}
	if strings.ContainsRune(options.connectionName, 0) {
		return options, nil, usageError("connection name contains a NUL byte", "Use a normal diagnostic name.")
	}
	if options.timeout <= 0 {
		return options, nil, usageError("timeout must be positive", "Use --timeout 30s.")
	}
	if options.human && len(options.fields) > 0 {
		return options, nil, usageError("--fields is available only in agent JSON mode", "Remove --human or use --agent with --fields.")
	}
	if options.quiet && len(options.fields) > 0 {
		return options, nil, usageError("--quiet and --fields cannot be combined", "Choose silent output or a filtered JSON response.")
	}
	return options, remaining, nil
}

func commandName(arguments []string) string {
	if len(arguments) == 0 {
		return "help"
	}
	best := arguments[0]
	for _, command := range commandCatalog {
		words := strings.Fields(command.Name)
		if len(words) > len(arguments) {
			continue
		}
		matched := true
		for index, word := range words {
			if arguments[index] != word {
				matched = false
				break
			}
		}
		if matched && len(command.Name) > len(best) {
			best = command.Name
		}
	}
	return best
}

func helpTopic(arguments []string) string {
	commandArguments := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if strings.HasPrefix(argument, "-") {
			break
		}
		commandArguments = append(commandArguments, argument)
	}
	if len(commandArguments) == 0 {
		return ""
	}
	return commandName(commandArguments)
}

func parseFieldSelection(value string) ([]string, error) {
	fields := make([]string, 0, 4)
	seen := make(map[string]struct{})
	for _, field := range strings.Split(value, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			return nil, usageError("--fields contains an empty field", "Use comma-separated names such as --fields pid,name.")
		}
		for _, segment := range strings.Split(field, ".") {
			if segment == "" {
				return nil, usageError("--fields contains an invalid path", "Use dotted names such as --fields process.architecture.")
			}
			for _, character := range segment {
				if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' && character != '-' {
					return nil, usageError(fmt.Sprintf("invalid --fields path %q", field), "Use letters, digits, underscores, hyphens, and dots only.")
				}
			}
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	return fields, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func parsePositiveInt32(value int, flagName string) (int32, error) {
	if value <= 0 || int64(value) > int64(^uint32(0)>>1) {
		return 0, usageError(flagName+" must be a positive 32-bit integer", "Provide a PID reported by 'cecli process list'.")
	}
	return int32(value), nil
}

func parseOptionalExitCode(value string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, usageError("invalid --exit-code", "Provide an integer exit code.")
	}
	return &parsed, nil
}
