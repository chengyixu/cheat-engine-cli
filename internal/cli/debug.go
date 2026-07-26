package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
	"github.com/chengyixu/cheat-engine-cli/internal/memory"
)

func (application *app) executeDebug(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("debug requires a subcommand", "Use trace, breakpoint, or context.")
	}
	switch arguments[0] {
	case "trace":
		return application.debugTrace(arguments[1:])
	case "breakpoint":
		return application.debugBreakpoint(arguments[1:])
	case "context":
		return application.debugContext(arguments[1:])
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown debug subcommand %q", arguments[0]), "Use trace, breakpoint, or context.")
	}
}

func (application *app) debugTrace(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("debug trace")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	maximumEvents := flagSet.Int("events", 10, "maximum events before detaching")
	eventTimeoutValue := flagSet.String("event-timeout", "1s", "timeout for each event wait")
	continueValue := flagSet.String("continue", "auto", "auto, deliver, ignore, or single-step")
	yes := flagSet.Bool("yes", false, "confirm temporary debugger attachment")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *maximumEvents < 1 || *maximumEvents > 10_000 {
		return commandResult{}, usageError("--events must be between 1 and 10000", "Use a bounded event count such as --events 10.")
	}
	eventTimeout, err := time.ParseDuration(*eventTimeoutValue)
	if err != nil || eventTimeout < time.Millisecond || eventTimeout > time.Duration(1<<31-1)*time.Millisecond {
		return commandResult{}, usageError("invalid --event-timeout", "Use a duration of at least 1ms that fits 32-bit milliseconds, such as 1s or 30s.")
	}
	if eventTimeout >= application.options.timeout {
		return commandResult{}, usageError("--event-timeout must be shorter than --timeout", "Increase the global network timeout or reduce the per-event timeout.")
	}
	continueMode, err := parseDebugContinueMode(*continueValue)
	if err != nil {
		return commandResult{}, err
	}
	preview := map[string]any{
		"pid": pid, "maximum_events": *maximumEvents, "event_timeout_ms": eventTimeout.Milliseconds(),
		"continue_mode": continueMode.String(), "dry_run": application.options.dryRun,
	}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nAttach to PID %d, collect at most %d events, wait %s per event, continue=%s", pid, *maximumEvents, eventTimeout, continueMode.String())}, nil
	}
	if err := requireYes(*yes, "debug trace attachment", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	trace, err := client.TraceDebugEvents(application.context, pid, *maximumEvents, eventTimeout, continueMode)
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{Data: trace, Human: renderDebugTrace(trace)}, nil
}

func (application *app) debugBreakpoint(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("debug breakpoint requires a subcommand", "Use set or remove.")
	}
	switch arguments[0] {
	case "set":
		return application.setBreakpoint(arguments[1:])
	case "remove":
		return application.removeBreakpoint(arguments[1:])
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown debug breakpoint subcommand %q", arguments[0]), "Use set or remove.")
	}
}

func (application *app) setBreakpoint(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("debug breakpoint set")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	tidValue := flagSet.Int("tid", -1, "target thread ID; -1 means all threads")
	registerValue := flagSet.Int("register", 0, "hardware debug-register index")
	addressValue := flagSet.String("address", "", "breakpoint address")
	kindValue := flagSet.String("kind", "execute", "execute, write, read, or access")
	sizeValue := flagSet.Int("size", 1, "breakpoint size: 1, 2, 4, or 8 bytes")
	yes := flagSet.Bool("yes", false, "confirm breakpoint change")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	tid, err := parseDebugThreadID(*tidValue)
	if err != nil {
		return commandResult{}, err
	}
	debugRegister, err := parseDebugRegister(*registerValue)
	if err != nil {
		return commandResult{}, err
	}
	if strings.TrimSpace(*addressValue) == "" {
		return commandResult{}, missingRequired("--address", "Use the authorized target address for the hardware breakpoint.")
	}
	address, err := memory.ParseAddress(*addressValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use a decimal or 0x-prefixed target address.")
	}
	breakpointType, err := parseBreakpointType(*kindValue)
	if err != nil {
		return commandResult{}, err
	}
	if *sizeValue != 1 && *sizeValue != 2 && *sizeValue != 4 && *sizeValue != 8 {
		return commandResult{}, usageError("--size must be 1, 2, 4, or 8", "Use a breakpoint size supported by the target architecture.")
	}
	preview := map[string]any{
		"pid": pid, "tid": tid, "debug_register": debugRegister, "address": fmt.Sprintf("0x%X", address),
		"kind": strings.ToLower(strings.TrimSpace(*kindValue)), "size": *sizeValue, "dry_run": application.options.dryRun,
	}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nSet %s breakpoint at 0x%X in register %d for PID %d / TID %d", preview["kind"], address, debugRegister, pid, tid)}, nil
	}
	if err := requireYes(*yes, "breakpoint set", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	changed, err := client.SetBreakpoint(pid, tid, debugRegister, address, breakpointType, int32(*sizeValue))
	if err != nil {
		return commandResult{}, err
	}
	if !changed {
		return commandResult{}, operationRejected("breakpoint set; an active ceserver debug session is required", preview)
	}
	preview["dry_run"] = false
	preview["changed"] = true
	return commandResult{Data: preview, Human: fmt.Sprintf("Set %s breakpoint at 0x%X in register %d", preview["kind"], address, debugRegister)}, nil
}

func (application *app) removeBreakpoint(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("debug breakpoint remove")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	tidValue := flagSet.Int("tid", -1, "target thread ID; -1 means all threads")
	registerValue := flagSet.Int("register", 0, "hardware debug-register index")
	watchpoint := flagSet.Bool("watchpoint", false, "remove a data watchpoint instead of an execute breakpoint")
	yes := flagSet.Bool("yes", false, "confirm breakpoint removal")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	tid, err := parseDebugThreadID(*tidValue)
	if err != nil {
		return commandResult{}, err
	}
	debugRegister, err := parseDebugRegister(*registerValue)
	if err != nil {
		return commandResult{}, err
	}
	preview := map[string]any{"pid": pid, "tid": tid, "debug_register": debugRegister, "watchpoint": *watchpoint, "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nRemove breakpoint in register %d for PID %d / TID %d", debugRegister, pid, tid)}, nil
	}
	if err := requireYes(*yes, "breakpoint removal", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	changed, err := client.RemoveBreakpoint(pid, tid, debugRegister, *watchpoint)
	if err != nil {
		return commandResult{}, err
	}
	if !changed {
		return commandResult{}, operationRejected("breakpoint removal; an active ceserver debug session is required", preview)
	}
	preview["dry_run"] = false
	preview["changed"] = true
	return commandResult{Data: preview, Human: fmt.Sprintf("Removed breakpoint in register %d", debugRegister)}, nil
}

func (application *app) debugContext(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("debug context requires a subcommand", "Use get or set.")
	}
	switch arguments[0] {
	case "get":
		return application.getThreadContext(arguments[1:])
	case "set":
		return application.setThreadContext(arguments[1:])
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown debug context subcommand %q", arguments[0]), "Use get or set.")
	}
}

func (application *app) getThreadContext(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("debug context get")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	tidValue := flagSet.Int("tid", 0, "target thread ID")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	tid, err := parsePositiveInt32(*tidValue, "--tid")
	if err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	threadContext, err := client.GetThreadContext(pid, tid)
	if err != nil {
		return commandResult{}, err
	}
	digest := sha256.Sum256(threadContext.Bytes)
	data := map[string]any{
		"pid": pid, "tid": tid, "struct_size": threadContext.StructSize, "type_code": threadContext.TypeCode,
		"bytes_base64": base64.StdEncoding.EncodeToString(threadContext.Bytes), "sha256": hex.EncodeToString(digest[:]),
	}
	human := fmt.Sprintf("PID: %d\nTID: %d\nSize: %d\nType: %d\nSHA-256: %s\nBase64: %s", pid, tid, threadContext.StructSize, threadContext.TypeCode, data["sha256"], data["bytes_base64"])
	return commandResult{Data: data, Human: human}, nil
}

func (application *app) setThreadContext(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("debug context set")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	tidValue := flagSet.Int("tid", 0, "target thread ID")
	base64Value := flagSet.String("base64", "", "raw context blob encoded as base64")
	hexValue := flagSet.String("hex", "", "raw context blob encoded as hexadecimal bytes")
	verify := flagSet.Bool("verify", false, "read back and compare the context blob")
	yes := flagSet.Bool("yes", false, "confirm thread-context replacement")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	tid, err := parsePositiveInt32(*tidValue, "--tid")
	if err != nil {
		return commandResult{}, err
	}
	contextBytes, err := decodeContextBytes(*base64Value, *hexValue)
	if err != nil {
		return commandResult{}, err
	}
	digest := sha256.Sum256(contextBytes)
	preview := map[string]any{
		"pid": pid, "tid": tid, "struct_size": len(contextBytes), "type_code": binary.LittleEndian.Uint32(contextBytes[4:8]),
		"sha256": hex.EncodeToString(digest[:]), "verify": *verify, "dry_run": application.options.dryRun,
	}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nReplace context for PID %d / TID %d with %d bytes (%s)", pid, tid, len(contextBytes), preview["sha256"])}, nil
	}
	if err := requireYes(*yes, "thread context replacement", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	changed, err := client.SetThreadContext(pid, tid, contextBytes)
	if err != nil {
		return commandResult{}, err
	}
	if !changed {
		return commandResult{}, operationRejected("thread context replacement", preview)
	}
	preview["dry_run"] = false
	preview["changed"] = true
	if *verify {
		readBack, err := client.GetThreadContext(pid, tid)
		if err != nil {
			return commandResult{}, err
		}
		preview["verified"] = bytes.Equal(readBack.Bytes, contextBytes)
		if preview["verified"] != true {
			return commandResult{}, &commandError{
				Code: "CONTEXT_VERIFICATION_FAILED", Message: "thread context read-back does not match the requested blob",
				Suggestion: "Keep the thread suspended, fetch the current context, and review architecture-specific changes before retrying.", ExitCode: 30, Details: preview,
			}
		}
	}
	return commandResult{Data: preview, Human: fmt.Sprintf("Replaced context for PID %d / TID %d with %d bytes", pid, tid, len(contextBytes))}, nil
}

func decodeContextBytes(base64Value, hexValue string) ([]byte, error) {
	if (strings.TrimSpace(base64Value) == "") == (strings.TrimSpace(hexValue) == "") {
		return nil, usageError("provide exactly one of --base64 or --hex", "Use the complete raw blob returned by 'cecli debug context get'.")
	}
	var data []byte
	var err error
	if strings.TrimSpace(base64Value) != "" {
		data, err = base64.StdEncoding.DecodeString(strings.TrimSpace(base64Value))
	} else {
		normalized := strings.NewReplacer(" ", "", "\n", "", "\r", "", "\t", "", "0x", "", "0X", "").Replace(hexValue)
		data, err = hex.DecodeString(normalized)
	}
	if err != nil {
		return nil, usageError("invalid context encoding", "Pass valid standard base64 or an even-length hexadecimal byte string.")
	}
	if len(data) < 8 || len(data) > 1<<20 {
		return nil, usageError("context blob must be between 8 and 1048576 bytes", "Use the complete blob returned by 'cecli debug context get'.")
	}
	declared := binary.LittleEndian.Uint32(data[:4])
	if uint64(declared) != uint64(len(data)) {
		return nil, usageError(fmt.Sprintf("context header declares %d bytes, input contains %d", declared, len(data)), "Do not edit or truncate the raw context header unless you understand the target ABI.")
	}
	return data, nil
}

func parseDebugContinueMode(value string) (ceserver.DebugContinueMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "safe":
		return ceserver.DebugContinueAuto, nil
	case "deliver", "signal", "0":
		return ceserver.DebugContinueDeliverSignal, nil
	case "ignore", "1":
		return ceserver.DebugContinueIgnoreSignal, nil
	case "single-step", "single_step", "step", "2":
		return ceserver.DebugContinueSingleStep, nil
	default:
		return 0, usageError("invalid --continue mode", "Use auto, deliver, ignore, or single-step.")
	}
}

func parseBreakpointType(value string) (ceserver.BreakpointType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "execute", "exec", "0":
		return ceserver.BreakpointExecute, nil
	case "write", "1":
		return ceserver.BreakpointWrite, nil
	case "read", "2":
		return ceserver.BreakpointRead, nil
	case "access", "readwrite", "read-write", "3":
		return ceserver.BreakpointAccess, nil
	default:
		return 0, usageError("invalid --kind", "Use execute, write, read, or access.")
	}
}

func parseDebugThreadID(value int) (int32, error) {
	if value == -1 {
		return -1, nil
	}
	return parsePositiveInt32(value, "--tid")
}

func parseDebugRegister(value int) (int32, error) {
	if value < 0 || value > 31 {
		return 0, usageError("--register must be between 0 and 31", "Use a debug-register index supported by the target architecture.")
	}
	return int32(value), nil
}

func renderDebugTrace(trace ceserver.DebugTrace) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "PID: %d\nEvents: %d/%d\nTimed out: %t\nContinue: %s", trace.PID, trace.EventCount, trace.MaximumEvents, trace.TimedOut, trace.ContinueModeName)
	for index, event := range trace.Events {
		fmt.Fprintf(&builder, "\n%d. %s TID=%d", index+1, event.SignalName, event.ThreadID)
		if event.Address != 0 {
			fmt.Fprintf(&builder, " address=0x%X", event.Address)
		}
		if event.Capabilities != nil {
			fmt.Fprintf(&builder, " breakpoints=%d watchpoints=%d shared=%d", event.Capabilities.Execute, event.Capabilities.Watch, event.Capabilities.Shared)
		}
	}
	return builder.String()
}
