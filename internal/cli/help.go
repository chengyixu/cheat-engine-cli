package cli

import (
	"fmt"
	"strconv"
	"strings"

	agentdocs "github.com/chengyixu/cheat-engine-cli"
)

type commandHelp struct {
	Name        string             `json:"name"`
	Usage       string             `json:"usage"`
	Description string             `json:"description"`
	Destructive bool               `json:"destructive"`
	Constraints []string           `json:"constraints,omitempty"`
	Parameters  []commandParameter `json:"parameters,omitempty"`
}

type commandParameter struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Enum     []any  `json:"enum,omitempty"`
	Default  any    `json:"default,omitempty"`
}

type globalOptionHelp struct {
	Flag        string `json:"flag"`
	Description string `json:"description"`
}

var globalOptionCatalog = []globalOptionHelp{
	{Flag: "--endpoint host:port", Description: "ceserver endpoint; env CECLI_ENDPOINT; default 127.0.0.1:52736"},
	{Flag: "--connection-name text", Description: "diagnostic name sent after connecting; env CECLI_CONNECTION_NAME"},
	{Flag: "--timeout duration", Description: "network timeout; env CECLI_TIMEOUT; default 30s"},
	{Flag: "--human", Description: "human-oriented terminal output"},
	{Flag: "--agent", Description: "force JSON output"},
	{Flag: "--pretty", Description: "indent JSON output"},
	{Flag: "--quiet, -q", Description: "suppress all output and rely on the exit code"},
	{Flag: "--fields path,...", Description: "return selected data fields; env CECLI_FIELDS"},
	{Flag: "--dry-run", Description: "preview destructive operations"},
	{Flag: "--help, -h", Description: "return full or command-specific help"},
	{Flag: "--version, -v", Description: "return version, commit, and build date"},
	{Flag: "--brief", Description: "return the embedded agent business card"},
}

var commandCatalog = []commandHelp{
	{Name: "server info", Usage: "cecli server info", Description: "Show ceserver protocol and ABI information."},
	{Name: "server path", Usage: "cecli server path", Description: "Show ceserver executable path, current path, and Android status."},
	{Name: "server connection-name", Usage: "cecli server connection-name --name text", Description: "Set the current ceserver connection's diagnostic name."},
	{Name: "server terminate", Usage: "cecli server terminate --yes", Description: "Terminate the remote ceserver process without a response.", Destructive: true, Constraints: []string{"the server stops immediately and all clients disconnect", "--yes is required unless --dry-run is used"}},
	{Name: "server options list", Usage: "cecli server options list", Description: "List ceserver runtime options."},
	{Name: "server options get", Usage: "cecli server options get --name text", Description: "Read one ceserver runtime option."},
	{Name: "server options set", Usage: "cecli server options set --name text --value text --yes", Description: "Change and verify one ceserver runtime option.", Destructive: true},
	{Name: "process list", Usage: "cecli process list [--filter text] [--limit n]", Description: "List processes visible to ceserver."},
	{Name: "process info", Usage: "cecli process info --pid n", Description: "Show architecture and inventory counts for a process."},
	{Name: "process speed", Usage: "cecli process speed --pid n --speed n --yes", Description: "Change target speed through the ceserver extension.", Destructive: true},
	{Name: "module list", Usage: "cecli module list --pid n [--filter text]", Description: "List mapped modules for a process."},
	{Name: "module load", Usage: "cecli module load --pid n --path remote.so --yes", Description: "Load a module from the target filesystem.", Destructive: true},
	{Name: "module load-ex", Usage: "cecli module load-ex --pid n --dlopen addr --path remote.so --yes", Description: "Load a target module using an explicit dlopen address.", Destructive: true},
	{Name: "module extension-load", Usage: "cecli module extension-load --pid n --yes", Description: "Inject the upstream ceserver extension into a target process.", Destructive: true},
	{Name: "thread list", Usage: "cecli thread list --pid n", Description: "List thread identifiers for a process."},
	{Name: "thread suspend", Usage: "cecli thread suspend --pid n --tid n --yes", Description: "Increment one thread's suspend count in an active debug session.", Destructive: true},
	{Name: "thread resume", Usage: "cecli thread resume --pid n --tid n --yes", Description: "Decrement one thread's suspend count in an active debug session.", Destructive: true},
	{Name: "thread create", Usage: "cecli thread create --pid n --start addr [--parameter addr] --yes", Description: "Create a remote target thread and return its handle.", Destructive: true},
	{Name: "thread close", Usage: "cecli thread close --handle n --yes", Description: "Close a remote thread handle.", Destructive: true},
	{Name: "debug trace", Usage: "cecli debug trace --pid n [--events n] [--event-timeout duration] [--continue auto|deliver|ignore|single-step] --yes", Description: "Attach for one bounded session, collect events, continue each event, and detach.", Destructive: true, Constraints: []string{"--event-timeout must be shorter than the global --timeout", "--yes is required unless --dry-run is used"}},
	{Name: "debug breakpoint set", Usage: "cecli debug breakpoint set --pid n --address addr [--tid n] [--register n] [--kind execute|write|read|access] [--size 1|2|4|8] --yes", Description: "Set a hardware breakpoint in an already-active ceserver debug session.", Destructive: true, Constraints: []string{"an active ceserver debug session is required", "--yes is required unless --dry-run is used"}},
	{Name: "debug breakpoint remove", Usage: "cecli debug breakpoint remove --pid n [--tid n] [--register n] [--watchpoint] --yes", Description: "Remove a hardware breakpoint from an active debug session.", Destructive: true, Constraints: []string{"an active ceserver debug session is required", "--yes is required unless --dry-run is used"}},
	{Name: "debug context get", Usage: "cecli debug context get --pid n --tid n", Description: "Export a validated raw ceserver thread-context blob."},
	{Name: "debug context set", Usage: "cecli debug context set --pid n --tid n (--base64 blob | --hex bytes) --yes [--verify]", Description: "Temporarily attach and replace a raw thread-context blob.", Destructive: true, Constraints: []string{"provide exactly one of --base64 or --hex", "--yes is required unless --dry-run is used"}},
	{Name: "memory regions", Usage: "cecli memory regions --pid n [--paged-only] [--dirty-only] [--no-shared]", Description: "List process memory regions."},
	{Name: "memory region", Usage: "cecli memory region --pid n --address addr", Description: "Inspect the region and maps line containing one address."},
	{Name: "memory read", Usage: "cecli memory read --pid n --address addr [--size n] [--format hex|base64|typed] [--type type]", Description: "Read and optionally decode process memory."},
	{Name: "memory scan", Usage: "cecli memory scan --pid n (--pattern bytes | --value value [--type type]) [--start addr] [--end addr] [--alignment n] [--protection readable|writable|executable|all|mask] [--limit n]", Description: "Scan readable process memory for an exact byte pattern or typed value.", Constraints: []string{"provide exactly one of --pattern or --value", "--type applies only when --value is used", "--start must be lower than --end"}},
	{Name: "memory aobscan", Usage: "cecli memory aobscan --pid n (--pattern bytes | --value value [--type type]) [--start addr] [--end addr] [--alignment n] [--protection readable|writable|executable|all|mask] [--limit n] --yes", Description: "Use the upstream experimental server-side AOB scanner.", Constraints: []string{"provide exactly one of --pattern or --value", "--type applies only when --value is used", "--start must be lower than --end", "--yes is required unless --dry-run is used", "use the default memory scan command unless server-side packet compatibility is specifically required"}},
	{Name: "memory write", Usage: "cecli memory write --pid n --address addr (--hex bytes | --value value [--type type]) --yes [--verify]", Description: "Write authorized process memory with explicit confirmation.", Destructive: true, Constraints: []string{"provide exactly one of --hex or --value", "--type applies only when --value is used", "--yes is required unless --dry-run is used"}},
	{Name: "memory alloc", Usage: "cecli memory alloc --pid n --size n [--address addr] [--protection noaccess|r|rw|rc|x|rx|rwx] --yes", Description: "Allocate target memory through ceserver.", Destructive: true, Constraints: []string{"--yes is required unless --dry-run is used"}},
	{Name: "memory free", Usage: "cecli memory free --pid n --address addr --size n --yes", Description: "Free a target allocation.", Destructive: true},
	{Name: "memory protect", Usage: "cecli memory protect --pid n --address addr --size n --protection rx --yes", Description: "Change target memory protection.", Destructive: true},
	{Name: "remote pwd", Usage: "cecli remote pwd", Description: "Show the ceserver current directory."},
	{Name: "remote cd", Usage: "cecli remote cd --path path --yes", Description: "Change the ceserver current directory.", Destructive: true},
	{Name: "remote ls", Usage: "cecli remote ls [--path path]", Description: "List a directory on the ceserver host."},
	{Name: "remote stat", Usage: "cecli remote stat --path path", Description: "Read remote path permissions."},
	{Name: "remote get", Usage: "cecli remote get --remote path --local path --yes [--force]", Description: "Download a bounded remote file to an explicitly confirmed local path.", Destructive: true},
	{Name: "remote put", Usage: "cecli remote put --local path --remote path --yes", Description: "Upload a bounded local file to the ceserver host.", Destructive: true},
	{Name: "remote mkdir", Usage: "cecli remote mkdir --path path --yes", Description: "Create a directory on the ceserver host.", Destructive: true},
	{Name: "remote rm", Usage: "cecli remote rm --path path --yes", Description: "Delete a path on the ceserver host.", Destructive: true},
	{Name: "remote chmod", Usage: "cecli remote chmod --path path --mode 0755 --yes", Description: "Change remote path permissions.", Destructive: true},
	{Name: "pipe open", Usage: "cecli pipe open --name text [--timeout-ms n]", Description: "Open a ceserver named pipe and return its handle."},
	{Name: "pipe read", Usage: "cecli pipe read --handle n --size n [--timeout-ms n] [--format hex|base64]", Description: "Read bounded bytes from a named pipe."},
	{Name: "pipe write", Usage: "cecli pipe write --handle n (--hex bytes | --text text | --file path) --yes [--timeout-ms n]", Description: "Write bounded bytes to a named pipe.", Destructive: true, Constraints: []string{"provide exactly one of --hex, --text, or --file", "--yes is required unless --dry-run is used"}},
	{Name: "pipe close", Usage: "cecli pipe close --handle n --yes", Description: "Close a named-pipe handle.", Destructive: true},
	{Name: "symbol list", Usage: "cecli symbol list --path remote-elf [--file-offset n] [--module-base addr] [--filter text] [--limit n]", Description: "Read and filter ELF symbols through ceserver."},
	{Name: "skills", Usage: "cecli skills [name]", Description: "List or show embedded agent usage skills."},
	{Name: "issue create", Usage: "cecli issue create --type bug|requirement|suggestion|bad-output --message text [--context json] [--exit-code n]", Description: "Create sanitized structured feedback in the local issue store."},
	{Name: "issue list", Usage: "cecli issue list [--status open|in-progress|resolved|closed] [--type bug|requirement|suggestion|bad-output]", Description: "List and filter local feedback."},
	{Name: "issue show", Usage: "cecli issue show ID", Description: "Show one local feedback record."},
	{Name: "issue transition", Usage: "cecli issue transition --status open|in-progress|resolved|closed ID", Description: "Change a local feedback record's status."},
	{Name: "completion", Usage: "cecli completion bash|zsh|fish", Description: "Generate shell completion source."},
	{Name: "self-check", Usage: "cecli self-check [--server]", Description: "Validate local contracts and optionally connect to ceserver."},
	{Name: "brief", Usage: "cecli brief", Description: "Return the embedded agent business card."},
	{Name: "version", Usage: "cecli version", Description: "Return version, commit, and build date."},
	{Name: "help", Usage: "cecli help [command]", Description: "Return the full or command-specific machine-readable help manifest."},
}

func (application *app) helpResult(topic string) commandResult {
	commands := enrichedCommandCatalog(commandCatalog)
	if topic != "" {
		filtered := make([]commandHelp, 0, 1)
		for _, command := range commands {
			if command.Name == topic || strings.HasPrefix(command.Name, topic+" ") {
				filtered = append(filtered, command)
			}
		}
		if len(filtered) > 0 {
			commands = filtered
		}
	}
	data := map[string]any{
		"schema_version": "cecli.help.v1", "name": "cecli", "version": Version,
		"brief": agentdocs.Brief(), "commands": commands,
		"success_schema": map[string]any{
			"type": "object", "required": []string{"ok", "command", "data", "meta", "rules", "skills", "issue"},
			"properties": map[string]string{"ok": "boolean:true", "command": "string", "data": "command-specific object", "meta": "object", "rules": "array", "skills": "array", "issue": "object"},
		},
		"error_schema": map[string]any{
			"type": "object", "required": []string{"error", "code", "message", "suggestion"},
			"properties": map[string]string{"error": "boolean:true", "code": "stable string", "message": "string", "suggestion": "string", "command": "string", "details": "object"},
		},
		"global_options": globalOptionCatalog,
	}
	var human strings.Builder
	human.WriteString("Cheat Engine CLI (cecli)\n\n")
	human.WriteString(agentdocs.Brief() + "\n\nUSAGE\n  cecli [global options] <command>\n\nCOMMANDS\n")
	for _, command := range commands {
		fmt.Fprintf(&human, "  %-18s %s\n", command.Name, command.Description)
	}
	human.WriteString("\nGLOBAL OPTIONS\n")
	for _, option := range globalOptionCatalog {
		fmt.Fprintf(&human, "  %-22s %s\n", option.Flag, option.Description)
	}
	return commandResult{Data: data, Human: human.String()}
}

func validateHelpCatalog() []string {
	var problems []string
	seenCommands := make(map[string]bool)
	for _, command := range commandCatalog {
		if strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Description) == "" || strings.TrimSpace(command.Usage) == "" {
			problems = append(problems, "every command needs a name, usage, and description")
			continue
		}
		if seenCommands[command.Name] {
			problems = append(problems, "duplicate command: "+command.Name)
		}
		seenCommands[command.Name] = true
		if !strings.HasPrefix(command.Usage, "cecli "+command.Name) {
			problems = append(problems, "usage does not match command: "+command.Name)
		}
		parameters := parametersFromUsage(command)
		if strings.TrimSpace(strings.TrimPrefix(command.Usage, "cecli "+command.Name)) != "" && len(parameters) == 0 {
			problems = append(problems, "usage has no typed parameters: "+command.Name)
		}
		seenParameters := make(map[string]bool)
		hasConfirmation := false
		for _, parameter := range parameters {
			if seenParameters[parameter.Name] {
				problems = append(problems, "duplicate parameter in "+command.Name+": "+parameter.Name)
			}
			seenParameters[parameter.Name] = true
			if parameter.Name == "yes" {
				hasConfirmation = true
			}
		}
		if command.Destructive && !hasConfirmation {
			problems = append(problems, "destructive command lacks --yes: "+command.Name)
		}
		if strings.Contains(command.Usage, " | ") && len(command.Constraints) == 0 {
			problems = append(problems, "alternative inputs lack constraints: "+command.Name)
		}
	}
	documentedGlobals := make(map[string]bool)
	for _, name := range globalOptionNames() {
		documentedGlobals[name] = true
	}
	for _, required := range []string{"--endpoint", "--connection-name", "--timeout", "--human", "--agent", "--pretty", "--quiet", "-q", "--fields", "--dry-run", "--help", "-h", "--version", "-v", "--brief"} {
		if !documentedGlobals[required] {
			problems = append(problems, "undocumented global option: "+required)
		}
	}
	return problems
}

func enrichedCommandCatalog(commands []commandHelp) []commandHelp {
	enriched := make([]commandHelp, len(commands))
	for index, command := range commands {
		enriched[index] = command
		enriched[index].Parameters = parametersFromUsage(command)
	}
	return enriched
}

func parametersFromUsage(command commandHelp) []commandParameter {
	prefix := "cecli " + command.Name
	remainder := strings.TrimSpace(strings.TrimPrefix(command.Usage, prefix))
	if remainder == "" {
		return nil
	}
	tokens := strings.Fields(remainder)
	alternativeFlags := alternativeFlagNames(remainder)
	parameters := make([]commandParameter, 0, len(tokens)/2+1)
	for index := 0; index < len(tokens); index++ {
		raw := tokens[index]
		cleaned := cleanUsageToken(raw)
		if cleaned == "" || cleaned == "|" {
			continue
		}
		optional := strings.Contains(raw, "[")
		if strings.HasPrefix(cleaned, "--") {
			name := strings.TrimPrefix(cleaned, "--")
			parameter := commandParameter{Name: name, Kind: "flag", Type: "boolean", Required: !optional && !alternativeFlags[name]}
			if index+1 < len(tokens) {
				nextRaw := tokens[index+1]
				next := cleanUsageToken(nextRaw)
				if next != "" && next != "|" && !strings.HasPrefix(next, "--") {
					parameter.Type, parameter.Enum = usageValueType(name, next)
					parameter.Required = parameter.Required && !strings.Contains(nextRaw, "[")
					index++
				}
			}
			if parameter.Type == "boolean" {
				parameter.Default = false
			}
			if value, ok := commandParameterDefault(command.Name, name); ok {
				parameter.Default = value
			}
			parameters = append(parameters, parameter)
			continue
		}
		parameterType, enum := usageValueType("", cleaned)
		parameters = append(parameters, commandParameter{Name: positionalParameterName(cleaned), Kind: "argument", Type: parameterType, Required: !optional, Enum: enum})
	}
	return parameters
}

func alternativeFlagNames(usage string) map[string]bool {
	names := make(map[string]bool)
	for offset := 0; offset < len(usage); {
		open := strings.IndexByte(usage[offset:], '(')
		if open < 0 {
			break
		}
		open += offset
		close := strings.IndexByte(usage[open+1:], ')')
		if close < 0 {
			break
		}
		close += open + 1
		group := usage[open+1 : close]
		if strings.Contains(group, " | ") {
			for _, token := range strings.Fields(group) {
				cleaned := cleanUsageToken(token)
				if strings.HasPrefix(cleaned, "--") {
					names[strings.TrimPrefix(cleaned, "--")] = true
				}
			}
		}
		offset = close + 1
	}
	return names
}

func commandParameterDefault(commandName, parameterName string) (any, bool) {
	defaults := map[string]any{
		"process list.filter":              "",
		"process list.limit":               0,
		"module list.filter":               "",
		"thread create.parameter":          "0",
		"debug trace.events":               10,
		"debug trace.event-timeout":        "1s",
		"debug trace.continue":             "auto",
		"debug breakpoint set.tid":         -1,
		"debug breakpoint set.register":    0,
		"debug breakpoint set.kind":        "execute",
		"debug breakpoint set.size":        1,
		"debug breakpoint remove.tid":      -1,
		"debug breakpoint remove.register": 0,
		"memory read.format":               "hex",
		"memory scan.type":                 "i32",
		"memory scan.start":                "0x0",
		"memory scan.end":                  "0x7FFFFFFFFFFF",
		"memory scan.alignment":            1,
		"memory scan.protection":           "readable",
		"memory scan.limit":                1000,
		"memory aobscan.type":              "i32",
		"memory aobscan.start":             "0x0",
		"memory aobscan.end":               "0x7FFFFFFFFFFF",
		"memory aobscan.alignment":         1,
		"memory aobscan.protection":        "readable",
		"memory aobscan.limit":             1000,
		"memory write.type":                "u32",
		"memory alloc.address":             "0",
		"memory alloc.protection":          "rw",
		"remote ls.path":                   ".",
		"pipe open.timeout-ms":             5000,
		"pipe read.timeout-ms":             5000,
		"pipe read.format":                 "hex",
		"pipe write.timeout-ms":            5000,
		"symbol list.file-offset":          "0",
		"symbol list.module-base":          "0",
		"symbol list.filter":               "",
		"symbol list.limit":                10000,
	}
	value, ok := defaults[commandName+"."+parameterName]
	return value, ok
}

func cleanUsageToken(value string) string {
	return strings.Trim(value, "[](),")
}

func usageValueType(name, placeholder string) (string, []any) {
	switch name {
	case "address", "start", "end", "parameter", "dlopen", "module-base", "handle":
		return "address", nil
	}
	if strings.Contains(placeholder, "|") {
		values := strings.Split(placeholder, "|")
		if enum, ok := integerEnum(values); ok {
			return "integer", enum
		}
		return "string", stringEnum(values)
	}
	switch placeholder {
	case "n", "0755":
		return "integer", nil
	case "addr":
		return "address", nil
	case "1s", "duration":
		return "duration", nil
	case "json":
		return "object", nil
	}
	switch name {
	case "pid", "tid", "limit", "size", "events", "timeout-ms", "file-offset", "exit-code", "register":
		return "integer", nil
	case "speed":
		return "number", nil
	}
	return "string", nil
}

func integerEnum(values []string) ([]any, bool) {
	enum := make([]any, len(values))
	for index, value := range values {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, false
		}
		enum[index] = parsed
	}
	return enum, len(enum) > 0
}

func stringEnum(values []string) []any {
	enum := make([]any, len(values))
	for index, value := range values {
		enum[index] = value
	}
	return enum
}

func positionalParameterName(placeholder string) string {
	if strings.Contains(placeholder, "|") {
		return "value"
	}
	switch placeholder {
	case "ID":
		return "id"
	case "name":
		return "name"
	case "command":
		return "command"
	default:
		return "value"
	}
}

func (application *app) versionResult() commandResult {
	data := map[string]string{"name": "cecli", "version": Version, "commit": Commit, "build_date": BuildDate}
	return commandResult{Data: data, Human: fmt.Sprintf("cecli %s (%s, %s)", Version, Commit, BuildDate)}
}

func (application *app) briefResult() commandResult {
	brief := agentdocs.Brief()
	return commandResult{Data: map[string]string{"brief": brief}, Human: brief}
}

func (application *app) executeCompletion(arguments []string) (commandResult, error) {
	if len(arguments) != 1 {
		return commandResult{}, usageError("completion requires one shell", "Use 'cecli completion bash', 'cecli completion zsh', or 'cecli completion fish'.")
	}
	script, err := completionScript(arguments[0])
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{Data: map[string]string{"shell": arguments[0], "script": script}, Human: script}, nil
}

func completionScript(shell string) (string, error) {
	contexts := completionContexts()
	switch shell {
	case "bash":
		return bashCompletionScript(contexts), nil
	case "zsh":
		return zshCompletionScript(contexts), nil
	case "fish":
		return fishCompletionScript(contexts), nil
	default:
		return "", usageError(fmt.Sprintf("unsupported shell %q", shell), "Supported shells: bash, zsh, fish.")
	}
}

type completionContext struct {
	Path       string
	Candidates []string
}

func completionContexts() []completionContext {
	contexts := []completionContext{{}}
	indexes := map[string]int{"": 0}
	for _, command := range enrichedCommandCatalog(commandCatalog) {
		words := strings.Fields(command.Name)
		for index, word := range words {
			path := strings.Join(words[:index], " ")
			contextIndex := ensureCompletionContext(&contexts, indexes, path)
			contexts[contextIndex].Candidates = appendUnique(contexts[contextIndex].Candidates, word)
			ensureCompletionContext(&contexts, indexes, strings.Join(words[:index+1], " "))
		}
		leafIndex := indexes[command.Name]
		for _, parameter := range command.Parameters {
			if parameter.Kind == "flag" {
				contexts[leafIndex].Candidates = appendUnique(contexts[leafIndex].Candidates, "--"+parameter.Name)
			} else {
				for _, value := range parameter.Enum {
					contexts[leafIndex].Candidates = appendUnique(contexts[leafIndex].Candidates, fmt.Sprint(value))
				}
			}
		}
	}
	globalFlags := globalOptionNames()
	for index := range contexts {
		contexts[index].Candidates = appendUnique(contexts[index].Candidates, globalFlags...)
	}
	return contexts
}

func globalOptionNames() []string {
	var names []string
	for _, option := range globalOptionCatalog {
		for _, field := range strings.Fields(strings.ReplaceAll(option.Flag, ",", "")) {
			if strings.HasPrefix(field, "-") {
				names = appendUnique(names, field)
			}
		}
	}
	return names
}

func ensureCompletionContext(contexts *[]completionContext, indexes map[string]int, path string) int {
	if index, ok := indexes[path]; ok {
		return index
	}
	index := len(*contexts)
	indexes[path] = index
	*contexts = append(*contexts, completionContext{Path: path})
	return index
}

func appendUnique(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		found := false
		for _, value := range values {
			if value == candidate {
				found = true
				break
			}
		}
		if !found {
			values = append(values, candidate)
		}
	}
	return values
}

func bashCompletionScript(contexts []completionContext) string {
	var builder strings.Builder
	builder.WriteString("_cecli_complete() {\n  local cur=\"${COMP_WORDS[COMP_CWORD]}\"\n  local context=\"\"\n  local candidates=\"\"\n  local i\n  for ((i=1; i<COMP_CWORD; i++)); do\n    [[ \"${COMP_WORDS[i]}\" == --* ]] && return 0\n    context+=\"${context:+ }${COMP_WORDS[i]}\"\n  done\n  case \"$context\" in\n")
	for _, context := range contexts {
		fmt.Fprintf(&builder, "    %s) candidates=%s ;;\n", shellSingleQuote(context.Path), shellSingleQuote(strings.Join(context.Candidates, " ")))
	}
	builder.WriteString("  esac\n  COMPREPLY=( $(compgen -W \"$candidates\" -- \"$cur\") )\n}\ncomplete -F _cecli_complete cecli")
	return builder.String()
}

func zshCompletionScript(contexts []completionContext) string {
	var builder strings.Builder
	builder.WriteString("#compdef cecli\n_cecli() {\n  local context=\"${(j: :)words[2,CURRENT-1]}\"\n  local -a candidates\n  case \"$context\" in\n")
	for _, context := range contexts {
		fmt.Fprintf(&builder, "    %s) candidates=(%s) ;;\n", shellSingleQuote(context.Path), shellQuotedWords(context.Candidates))
	}
	builder.WriteString("  esac\n  _describe 'cecli value' candidates\n}\ncompdef _cecli cecli")
	return builder.String()
}

func fishCompletionScript(contexts []completionContext) string {
	var builder strings.Builder
	builder.WriteString("function __cecli_path_is\n  set -l tokens (commandline -opc)\n  if test (count $tokens) -gt 0\n    set -e tokens[1]\n  end\n  test (string join ' ' -- $tokens) = (string join ' ' -- $argv)\nend\n")
	for _, context := range contexts {
		condition := "__cecli_path_is"
		if context.Path != "" {
			condition += " " + context.Path
		}
		fmt.Fprintf(&builder, "complete -c cecli -f -n %s -a %s\n", shellSingleQuote(condition), shellSingleQuote(strings.Join(context.Candidates, " ")))
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

func shellQuotedWords(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = shellSingleQuote(value)
	}
	return strings.Join(quoted, " ")
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
