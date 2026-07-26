package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
)

func TestHelpIsJSONByDefault(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %s", stderr.String())
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output["ok"] != true || output["command"] != "help" {
		t.Fatalf("output = %#v", output)
	}
}

func TestNativeAndEndpointAreMutuallyExclusive(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"--native", "--endpoint", "127.0.0.1:52736", "process", "list"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "--native and --endpoint cannot be combined") {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestNativeEnvironmentValidation(t *testing.T) {
	t.Setenv("CECLI_NATIVE", "sometimes")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"process", "list"}, &stdout, &stderr)
	if exitCode != 2 || !strings.Contains(stderr.String(), "invalid CECLI_NATIVE") {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
}

func TestNativeOpenFailureHasPlatformGuidance(t *testing.T) {
	application := &app{options: globalOptions{native: true}}
	normalized := normalizeError(&ceserver.ProtocolError{Operation: "open process", Message: "server denied or could not open PID 42"})
	if normalized.Code != "CESERVER_PROTOCOL_ERROR" {
		t.Fatalf("unexpected starting code %q", normalized.Code)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application.stdout = &stdout
	application.stderr = &stderr
	exitCode := application.writeError("memory read", &ceserver.ProtocolError{Operation: "open process", Message: "server denied or could not open PID 42"})
	if exitCode != 1 || !strings.Contains(stderr.String(), "NATIVE_PROCESS_UNAVAILABLE") || !strings.Contains(stderr.String(), "permissions") {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
}

func TestCommandHelpUsesCommandTopic(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"debug", "context", "set", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
	var output struct {
		Command string `json:"command"`
		Data    struct {
			Commands []struct {
				Name string `json:"name"`
			} `json:"commands"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Command != "debug context set" || len(output.Data.Commands) != 1 || output.Data.Commands[0].Name != "debug context set" {
		t.Fatalf("output = %#v", output)
	}
}

func TestHelpMarksConditionalFlagsOptional(t *testing.T) {
	commands := enrichedCommandCatalog(commandCatalog)
	scan := findCommandHelp(t, commands, "memory scan")
	for _, name := range []string{"pattern", "value", "type"} {
		if parameter := findCommandParameter(t, scan, name); parameter.Required {
			t.Fatalf("memory scan --%s must be conditional, not universally required: %#v", name, parameter)
		}
	}
	contextSet := findCommandHelp(t, commands, "debug context set")
	for _, name := range []string{"base64", "hex"} {
		if parameter := findCommandParameter(t, contextSet, name); parameter.Required {
			t.Fatalf("debug context set --%s must be conditional, not universally required: %#v", name, parameter)
		}
	}
	if len(scan.Constraints) == 0 || len(contextSet.Constraints) == 0 {
		t.Fatal("conditional commands must expose machine-readable constraints")
	}
	serverScan := findCommandHelp(t, commands, "memory aobscan")
	for _, name := range []string{"pattern", "value", "type"} {
		if parameter := findCommandParameter(t, serverScan, name); parameter.Required {
			t.Fatalf("memory aobscan --%s must be conditional, not universally required: %#v", name, parameter)
		}
	}
}

func TestHelpEmitsDefaultsAndEnums(t *testing.T) {
	commands := enrichedCommandCatalog(commandCatalog)
	trace := findCommandHelp(t, commands, "debug trace")
	continueParameter := findCommandParameter(t, trace, "continue")
	if continueParameter.Default != "auto" || fmt.Sprint(continueParameter.Enum) != "[auto deliver ignore single-step]" {
		t.Fatalf("continue parameter = %#v", continueParameter)
	}
	breakpoint := findCommandHelp(t, commands, "debug breakpoint set")
	size := findCommandParameter(t, breakpoint, "size")
	if size.Type != "integer" || size.Default != 1 || fmt.Sprint(size.Enum) != "[1 2 4 8]" {
		t.Fatalf("size parameter = %#v", size)
	}
	scan := findCommandHelp(t, commands, "memory scan")
	if limit := findCommandParameter(t, scan, "limit"); limit.Default != 1000 {
		t.Fatalf("limit parameter = %#v", limit)
	}
}

func TestCompletionsIncludeNestedCommandsAndFlags(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%q): %v", shell, err)
		}
		for _, expected := range []string{"server options", "debug breakpoint set", "memory scan", "--pattern", "--continue"} {
			if !strings.Contains(script, expected) {
				t.Fatalf("%s completion missing %q", shell, expected)
			}
		}
	}
}

func TestHelpCatalogSelfValidation(t *testing.T) {
	if problems := validateHelpCatalog(); len(problems) > 0 {
		t.Fatalf("help catalog problems: %v", problems)
	}
}

func findCommandHelp(t *testing.T, commands []commandHelp, name string) commandHelp {
	t.Helper()
	for _, command := range commands {
		if command.Name == name {
			return command
		}
	}
	t.Fatalf("command %q not found", name)
	return commandHelp{}
}

func findCommandParameter(t *testing.T, command commandHelp, name string) commandParameter {
	t.Helper()
	for _, parameter := range command.Parameters {
		if parameter.Name == name {
			return parameter
		}
	}
	t.Fatalf("parameter %q not found for command %q", name, command.Name)
	return commandParameter{}
}

func TestFieldsFiltersData(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"version", "--fields", "name,version,missing"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
	var output struct {
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Data["name"] != "cecli" || output.Data["version"] == nil || len(output.Data) != 2 {
		t.Fatalf("data = %#v", output.Data)
	}
	missing, _ := output.Meta["missing_fields"].([]any)
	if len(missing) != 1 || missing[0] != "missing" {
		t.Fatalf("meta = %#v", output.Meta)
	}
}

func TestQuietSuppressesSuccessAndErrorOutput(t *testing.T) {
	for _, arguments := range [][]string{{"version", "--quiet"}, {"memory", "read", "--pid", "1", "--quiet"}} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := Run(context.Background(), arguments, &stdout, &stderr)
		if arguments[0] == "version" && exitCode != 0 {
			t.Fatalf("success exitCode = %d", exitCode)
		}
		if arguments[0] == "memory" && exitCode != 2 {
			t.Fatalf("error exitCode = %d", exitCode)
		}
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
		}
	}
}

func TestUsageErrorWritesOnlyStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"memory", "read", "--pid", "1"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %s", stdout.String())
	}
	var output map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output["code"] != "MISSING_REQUIRED" {
		t.Fatalf("output = %#v", output)
	}
}

func TestDryRunWriteDoesNotConnect(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	arguments := []string{"memory", "write", "--pid", "42", "--address", "0x1000", "--type", "i32", "--value", "100", "--dry-run"}
	exitCode := Run(context.Background(), arguments, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
	}
	var output struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Data["dry_run"] != true || output.Data["bytes_hex"] != "64000000" {
		t.Fatalf("data = %#v", output.Data)
	}
}

func TestExpandedMutationsDryRunWithoutServer(t *testing.T) {
	temporaryFile := t.TempDir() + "/payload.bin"
	if err := os.WriteFile(temporaryFile, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	testCases := [][]string{
		{"memory", "alloc", "--pid", "42", "--size", "4096", "--protection", "rw", "--dry-run"},
		{"memory", "protect", "--pid", "42", "--address", "0x1000", "--size", "4096", "--protection", "rx", "--dry-run"},
		{"process", "speed", "--pid", "42", "--speed", "1.5", "--dry-run"},
		{"module", "load", "--pid", "42", "--path", "/tmp/plugin.so", "--dry-run"},
		{"module", "load-ex", "--pid", "42", "--dlopen", "0x1234", "--path", "/tmp/plugin.so", "--dry-run"},
		{"module", "extension-load", "--pid", "42", "--dry-run"},
		{"remote", "put", "--local", temporaryFile, "--remote", "/tmp/payload.bin", "--dry-run"},
		{"remote", "get", "--remote", "/tmp/payload.bin", "--local", temporaryFile + ".download", "--dry-run"},
		{"server", "options", "set", "--name", "optMSO", "--value", "2", "--dry-run"},
		{"server", "terminate", "--dry-run"},
		{"thread", "create", "--pid", "42", "--start", "0x1000", "--parameter", "0x2000", "--dry-run"},
		{"thread", "suspend", "--pid", "42", "--tid", "43", "--dry-run"},
		{"thread", "resume", "--pid", "42", "--tid", "43", "--dry-run"},
		{"debug", "trace", "--pid", "42", "--events", "5", "--event-timeout", "250ms", "--dry-run"},
		{"debug", "breakpoint", "set", "--pid", "42", "--address", "0x1000", "--dry-run"},
		{"debug", "breakpoint", "remove", "--pid", "42", "--register", "0", "--dry-run"},
		{"debug", "context", "set", "--pid", "42", "--tid", "43", "--base64", "EAAAAAMAAAAAAQIDBAUGBw==", "--dry-run"},
		{"pipe", "write", "--handle", "81", "--text", "hello", "--dry-run"},
		{"memory", "aobscan", "--pid", "42", "--pattern", "48 8B ?? FF", "--start", "0x1000", "--end", "0x2000", "--dry-run"},
	}
	for _, arguments := range testCases {
		t.Run(strings.Join(arguments[:2], "_"), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run(context.Background(), arguments, &stdout, &stderr)
			if exitCode != 0 {
				t.Fatalf("exitCode = %d, stderr = %s", exitCode, stderr.String())
			}
			var output struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			if output.Data["dry_run"] != true {
				t.Fatalf("data = %#v", output.Data)
			}
		})
	}
}
