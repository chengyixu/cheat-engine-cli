package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	agentdocs "github.com/chengyixu/cheat-engine-cli"
	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
	"github.com/chengyixu/cheat-engine-cli/internal/feedback"
	"github.com/chengyixu/cheat-engine-cli/internal/memory"
)

const (
	maximumReadSize     = 16 << 20
	maximumWriteSize    = 1 << 20
	maximumTransferSize = 256 << 20
)

func (application *app) executeServer(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("server requires a subcommand", "Use info, path, connection-name, options, or terminate.")
	}
	switch arguments[0] {
	case "info":
		if len(arguments) != 1 {
			return commandResult{}, usageError("server info accepts no arguments", "Use 'cecli server info'.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		serverInfo, err := client.ServerInfo()
		if err != nil {
			return commandResult{}, err
		}
		human := fmt.Sprintf("Endpoint: %s\nProtocol: %d\nVersion:  %s\nABI:      %s", serverInfo.Endpoint, serverInfo.ProtocolVersion, serverInfo.VersionName, serverInfo.ABI)
		return commandResult{Data: serverInfo, Human: human}, nil
	case "path":
		if len(arguments) != 1 {
			return commandResult{}, usageError("server path accepts no arguments", "Use 'cecli server path'.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		pathInfo, err := client.PathInfo()
		if err != nil {
			return commandResult{}, err
		}
		human := fmt.Sprintf("Executable: %s\nCurrent:    %s\nAndroid:    %t", pathInfo.ExecutablePath, pathInfo.CurrentPath, pathInfo.Android)
		return commandResult{Data: pathInfo, Human: human}, nil
	case "connection-name":
		flagSet := newFlagSet("server connection-name")
		name := flagSet.String("name", "", "diagnostic connection name")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if strings.TrimSpace(*name) == "" || strings.ContainsRune(*name, 0) {
			return commandResult{}, missingRequired("--name", "Use a non-empty diagnostic name such as ci-worker-1.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		if err := client.SetConnectionName(*name); err != nil {
			return commandResult{}, err
		}
		return commandResult{Data: map[string]string{"name": *name}, Human: "Connection name: " + *name}, nil
	case "options":
		return application.serverOptions(arguments[1:])
	case "terminate":
		flagSet := newFlagSet("server terminate")
		yes := flagSet.Bool("yes", false, "confirm remote server termination")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"endpoint": application.options.endpoint, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: "DRY RUN\nTerminate ceserver at " + application.options.endpoint}, nil
		}
		if err := requireYes(*yes, "ceserver termination", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		if err := client.TerminateServer(); err != nil {
			return commandResult{}, err
		}
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: "Termination command sent to " + application.options.endpoint}, nil
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown server subcommand %q", arguments[0]), "Use info, path, connection-name, options, or terminate.")
	}
}

func (application *app) serverOptions(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("server options requires a subcommand", "Use list, get, or set.")
	}
	switch arguments[0] {
	case "list":
		if len(arguments) != 1 {
			return commandResult{}, usageError("server options list accepts no arguments", "Use 'cecli server options list'.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		options, err := client.Options()
		if err != nil {
			return commandResult{}, err
		}
		return commandResult{Data: map[string]any{"options": options, "count": len(options)}, Human: renderServerOptions(options)}, nil
	case "get":
		flagSet := newFlagSet("server options get")
		name := flagSet.String("name", "", "option name")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if *name == "" {
			return commandResult{}, missingRequired("--name", "Run 'cecli server options list' and pass an option name.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		value, err := client.Option(*name)
		if err != nil {
			return commandResult{}, err
		}
		return commandResult{Data: map[string]string{"name": *name, "value": value}, Human: fmt.Sprintf("%s=%s", *name, value)}, nil
	case "set":
		flagSet := newFlagSet("server options set")
		name := flagSet.String("name", "", "option name")
		value := flagSet.String("value", "", "new option value")
		yes := flagSet.Bool("yes", false, "confirm the change")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if *name == "" {
			return commandResult{}, missingRequired("--name", "Run 'cecli server options list' and pass an option name.")
		}
		if !flagWasSet(flagSet, "value") {
			return commandResult{}, missingRequired("--value", "Use --value with the new option value.")
		}
		preview := map[string]any{"name": *name, "value": *value, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\n%s=%s", *name, *value)}, nil
		}
		if err := requireYes(*yes, "server option change", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		if err := client.SetOption(*name, *value); err != nil {
			return commandResult{}, err
		}
		readBack, err := client.Option(*name)
		if err != nil {
			return commandResult{}, err
		}
		preview["dry_run"] = false
		preview["read_back"] = readBack
		preview["verified"] = readBack == *value
		if readBack != *value {
			return commandResult{}, &commandError{
				Code: "OPTION_VERIFICATION_FAILED", Message: "server option read-back does not match the requested value",
				Suggestion: "Inspect acceptable values and the server option type, then retry with a valid value.", ExitCode: 30, Details: preview,
			}
		}
		return commandResult{Data: preview, Human: fmt.Sprintf("%s=%s\nRead back: %s", *name, *value, readBack)}, nil
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown server options subcommand %q", arguments[0]), "Use list, get, or set.")
	}
}

func (application *app) executeProcess(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("process requires a subcommand", "Use 'cecli process list' or 'cecli process info --pid <pid>'.")
	}
	switch arguments[0] {
	case "list":
		flagSet := newFlagSet("process list")
		filterValue := flagSet.String("filter", "", "case-insensitive process name filter")
		limit := flagSet.Int("limit", 0, "maximum number of processes; 0 means all")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if *limit < 0 {
			return commandResult{}, usageError("--limit cannot be negative", "Use --limit 100 or omit it.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		processes, err := client.ListProcesses()
		if err != nil {
			return commandResult{}, err
		}
		filtered := make([]ceserver.Process, 0, len(processes))
		for _, process := range processes {
			if *filterValue != "" && !strings.Contains(strings.ToLower(process.Name), strings.ToLower(*filterValue)) {
				continue
			}
			filtered = append(filtered, process)
			if *limit > 0 && len(filtered) >= *limit {
				break
			}
		}
		return commandResult{Data: map[string]any{"processes": filtered, "count": len(filtered)}, Human: renderProcesses(filtered)}, nil
	case "info":
		flagSet := newFlagSet("process info")
		pidValue := flagSet.Int("pid", 0, "target process ID")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		pid, err := parsePositiveInt32(*pidValue, "--pid")
		if err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		processInfo, err := client.ProcessInfo(pid)
		if err != nil {
			return commandResult{}, err
		}
		human := fmt.Sprintf("PID:          %d\nArchitecture: %s\nModules:      %d\nThreads:      %d", processInfo.PID, processInfo.Architecture, processInfo.ModuleCount, processInfo.ThreadCount)
		return commandResult{Data: processInfo, Human: human}, nil
	case "speed":
		flagSet := newFlagSet("process speed")
		pidValue := flagSet.Int("pid", 0, "target process ID")
		speedValue := flagSet.Float64("speed", 0, "positive speed multiplier")
		yes := flagSet.Bool("yes", false, "confirm the change")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		pid, err := parsePositiveInt32(*pidValue, "--pid")
		if err != nil {
			return commandResult{}, err
		}
		if *speedValue <= 0 {
			return commandResult{}, usageError("--speed must be positive", "Use --speed 0.5, 1, or 2.")
		}
		preview := map[string]any{"pid": pid, "speed": *speedValue, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nPID: %d\nSpeed: %g", pid, *speedValue)}, nil
		}
		if err := requireYes(*yes, "process speed change", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		changed, err := client.SetSpeed(pid, float32(*speedValue))
		if err != nil {
			return commandResult{}, err
		}
		if !changed {
			return commandResult{}, operationRejected("process speed change", preview)
		}
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: fmt.Sprintf("PID %d speed set to %g", pid, *speedValue)}, nil
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown process subcommand %q", arguments[0]), "Use 'cecli process list' or 'cecli process info'.")
	}
}

func (application *app) executeModule(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("module requires a subcommand", "Use list, load, load-ex, or extension-load.")
	}
	if arguments[0] == "extension-load" {
		flagSet := newFlagSet("module extension-load")
		pidValue := flagSet.Int("pid", 0, "target process ID")
		yes := flagSet.Bool("yes", false, "confirm ceserver extension loading")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		pid, err := parsePositiveInt32(*pidValue, "--pid")
		if err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"pid": pid, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nLoad the ceserver extension into PID %d", pid)}, nil
		}
		if err := requireYes(*yes, "ceserver extension load", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		changed, err := client.LoadExtension(pid)
		if err != nil {
			return commandResult{}, err
		}
		if !changed {
			return commandResult{}, operationRejected("ceserver extension load", preview)
		}
		preview["dry_run"] = false
		preview["loaded"] = true
		return commandResult{Data: preview, Human: fmt.Sprintf("Loaded the ceserver extension into PID %d", pid)}, nil
	}
	if arguments[0] == "load-ex" {
		flagSet := newFlagSet("module load-ex")
		pidValue := flagSet.Int("pid", 0, "target process ID")
		dlopenValue := flagSet.String("dlopen", "", "target dlopen address")
		remotePath := flagSet.String("path", "", "module path on the ceserver target")
		yes := flagSet.Bool("yes", false, "confirm module loading")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		pid, err := parsePositiveInt32(*pidValue, "--pid")
		if err != nil {
			return commandResult{}, err
		}
		if strings.TrimSpace(*dlopenValue) == "" {
			return commandResult{}, missingRequired("--dlopen", "Use the resolved dlopen address for the target architecture.")
		}
		dlopenAddress, err := memory.ParseAddress(*dlopenValue)
		if err != nil {
			return commandResult{}, usageError(err.Error(), "Use a decimal or 0x-prefixed target address.")
		}
		if err := validateRemotePath(*remotePath); err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"pid": pid, "dlopen": fmt.Sprintf("0x%X", dlopenAddress), "path": *remotePath, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nLoad %s into PID %d using dlopen at 0x%X", *remotePath, pid, dlopenAddress)}, nil
		}
		if err := requireYes(*yes, "explicit-dlopen module load", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		address, err := client.LoadModuleEx(pid, dlopenAddress, *remotePath)
		if err != nil {
			return commandResult{}, err
		}
		preview["address"] = fmt.Sprintf("0x%X", address)
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: fmt.Sprintf("Loaded %s at 0x%X", *remotePath, address)}, nil
	}
	if arguments[0] == "load" {
		flagSet := newFlagSet("module load")
		pidValue := flagSet.Int("pid", 0, "target process ID")
		remotePath := flagSet.String("path", "", "module path on the ceserver target")
		yes := flagSet.Bool("yes", false, "confirm module loading")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		pid, err := parsePositiveInt32(*pidValue, "--pid")
		if err != nil {
			return commandResult{}, err
		}
		if err := validateRemotePath(*remotePath); err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"pid": pid, "path": *remotePath, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nPID: %d\nModule: %s", pid, *remotePath)}, nil
		}
		if err := requireYes(*yes, "remote module load", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		address, err := client.LoadModule(pid, *remotePath)
		if err != nil {
			return commandResult{}, err
		}
		preview["address"] = fmt.Sprintf("0x%X", address)
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: fmt.Sprintf("Loaded %s at 0x%X", *remotePath, address)}, nil
	}
	if arguments[0] != "list" {
		return commandResult{}, usageError(fmt.Sprintf("unknown module subcommand %q", arguments[0]), "Use list, load, load-ex, or extension-load.")
	}
	flagSet := newFlagSet("module list")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	filterValue := flagSet.String("filter", "", "case-insensitive module name filter")
	if err := parseFlags(flagSet, arguments[1:]); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	modules, err := client.ListModules(pid)
	if err != nil {
		return commandResult{}, err
	}
	filtered := make([]ceserver.Module, 0, len(modules))
	for _, module := range modules {
		if *filterValue == "" || strings.Contains(strings.ToLower(module.Name), strings.ToLower(*filterValue)) {
			filtered = append(filtered, module)
		}
	}
	return commandResult{Data: map[string]any{"pid": pid, "modules": filtered, "count": len(filtered)}, Human: renderModules(filtered)}, nil
}

func (application *app) executeThread(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("thread requires a subcommand", "Use list, suspend, resume, create, or close.")
	}
	if arguments[0] == "suspend" || arguments[0] == "resume" {
		return application.changeThreadSuspension(arguments[0], arguments[1:])
	}
	if arguments[0] == "create" {
		flagSet := newFlagSet("thread create")
		pidValue := flagSet.Int("pid", 0, "target process ID")
		startValue := flagSet.String("start", "", "thread start address")
		parameterValue := flagSet.String("parameter", "0", "thread parameter address")
		yes := flagSet.Bool("yes", false, "confirm remote thread creation")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		pid, err := parsePositiveInt32(*pidValue, "--pid")
		if err != nil {
			return commandResult{}, err
		}
		if *startValue == "" {
			return commandResult{}, missingRequired("--start", "Use an authorized executable address in the target process.")
		}
		startAddress, err := memory.ParseAddress(*startValue)
		if err != nil {
			return commandResult{}, usageError(err.Error(), "Use a valid target start address.")
		}
		parameter, err := memory.ParseAddress(*parameterValue)
		if err != nil {
			return commandResult{}, usageError(err.Error(), "Use --parameter 0 or a valid target address.")
		}
		preview := map[string]any{"pid": pid, "start": fmt.Sprintf("0x%X", startAddress), "parameter": fmt.Sprintf("0x%X", parameter), "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nCreate thread in PID %d at 0x%X with parameter 0x%X", pid, startAddress, parameter)}, nil
		}
		if err := requireYes(*yes, "remote thread creation", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		handle, err := client.CreateRemoteThread(pid, startAddress, parameter)
		if err != nil {
			return commandResult{}, err
		}
		preview["handle"] = handle
		preview["handle_hex"] = fmt.Sprintf("0x%X", handle)
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: fmt.Sprintf("Created remote thread handle %d (0x%X)", handle, handle)}, nil
	}
	if arguments[0] == "close" {
		flagSet := newFlagSet("thread close")
		handleValue := flagSet.String("handle", "", "remote thread handle")
		yes := flagSet.Bool("yes", false, "confirm handle close")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		handle, err := parseHandle(*handleValue)
		if err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"handle": handle, "handle_hex": fmt.Sprintf("0x%X", handle), "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nClose thread handle %d", handle)}, nil
		}
		if err := requireYes(*yes, "thread handle close", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		if err := client.CloseHandle(handle); err != nil {
			return commandResult{}, err
		}
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: fmt.Sprintf("Closed thread handle %d", handle)}, nil
	}
	if arguments[0] != "list" {
		return commandResult{}, usageError(fmt.Sprintf("unknown thread subcommand %q", arguments[0]), "Use list, suspend, resume, create, or close.")
	}
	flagSet := newFlagSet("thread list")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	if err := parseFlags(flagSet, arguments[1:]); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	threads, err := client.ListThreads(pid)
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{Data: map[string]any{"pid": pid, "threads": threads, "count": len(threads)}, Human: renderThreads(threads)}, nil
}

func (application *app) changeThreadSuspension(action string, arguments []string) (commandResult, error) {
	flagSet := newFlagSet("thread " + action)
	pidValue := flagSet.Int("pid", 0, "target process ID")
	tidValue := flagSet.Int("tid", 0, "target thread ID")
	yes := flagSet.Bool("yes", false, "confirm thread state change")
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
	preview := map[string]any{"pid": pid, "tid": tid, "action": action, "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\n%s TID %d in PID %d", strings.ToUpper(action[:1])+action[1:], tid, pid)}, nil
	}
	if err := requireYes(*yes, "thread "+action, preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	var suspendCount int32
	if action == "suspend" {
		suspendCount, err = client.SuspendThread(pid, tid)
	} else {
		suspendCount, err = client.ResumeThread(pid, tid)
	}
	if err != nil {
		return commandResult{}, err
	}
	preview["dry_run"] = false
	preview["suspend_count"] = suspendCount
	return commandResult{Data: preview, Human: fmt.Sprintf("%s TID %d in PID %d; suspend count is %d", strings.ToUpper(action[:1])+action[1:], tid, pid, suspendCount)}, nil
}

func (application *app) executeMemory(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("memory requires a subcommand", "Use 'cecli help memory'.")
	}
	switch arguments[0] {
	case "regions":
		return application.memoryRegions(arguments[1:])
	case "region":
		return application.memoryRegion(arguments[1:])
	case "read":
		return application.memoryRead(arguments[1:])
	case "write":
		return application.memoryWrite(arguments[1:])
	case "scan":
		return application.memoryScan(arguments[1:])
	case "aobscan":
		return application.memoryAOBScan(arguments[1:])
	case "alloc":
		return application.memoryAllocate(arguments[1:])
	case "free":
		return application.memoryFree(arguments[1:])
	case "protect":
		return application.memoryProtect(arguments[1:])
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown memory subcommand %q", arguments[0]), "Use 'cecli help memory'.")
	}
}

func (application *app) memoryRegion(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory region")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	addressValue := flagSet.String("address", "", "address in decimal or 0x notation")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *addressValue == "" {
		return commandResult{}, missingRequired("--address", "Use an address to resolve to one mapped region.")
	}
	address, err := memory.ParseAddress(*addressValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use a decimal or 0x-prefixed address.")
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	region, err := client.RegionInfo(pid, address)
	if err != nil {
		return commandResult{}, err
	}
	data := map[string]any{"pid": pid, "address": fmt.Sprintf("0x%X", address), "region": region}
	human := fmt.Sprintf("Base: 0x%X\nEnd:  0x%X\nSize: 0x%X\nPerm: %s\nType: %s\nMap:  %s", region.BaseAddress, region.BaseAddress+region.Size, region.Size, region.Permissions, region.TypeName, region.MapsLine)
	return commandResult{Data: data, Human: human}, nil
}

func (application *app) memoryAllocate(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory alloc")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	preferredValue := flagSet.String("address", "0", "preferred base address; 0 lets the target choose")
	sizeValue := flagSet.Uint("size", 0, "allocation size in bytes")
	protectionValue := flagSet.String("protection", "rw", "noaccess, r, rw, rc, x, rx, or rwx")
	yes := flagSet.Bool("yes", false, "confirm allocation")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *sizeValue == 0 || uint64(*sizeValue) > uint64(^uint32(0)) {
		return commandResult{}, usageError("--size must be between 1 and 4294967295", "Use a bounded allocation size such as --size 4096.")
	}
	preferredAddress, err := memory.ParseAddress(*preferredValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use --address 0 or a valid target address.")
	}
	protection, err := parsePageProtection(*protectionValue)
	if err != nil {
		return commandResult{}, err
	}
	preview := map[string]any{"pid": pid, "preferred_address": fmt.Sprintf("0x%X", preferredAddress), "size": *sizeValue, "protection": protection.String(), "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nPID: %d\nSize: %d\nProtection: %s", pid, *sizeValue, protection.String())}, nil
	}
	if err := requireYes(*yes, "memory allocation", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	address, err := client.AllocateMemory(pid, preferredAddress, uint32(*sizeValue), protection)
	if err != nil {
		return commandResult{}, err
	}
	preview["address"] = fmt.Sprintf("0x%X", address)
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("Allocated %d bytes at 0x%X (%s)", *sizeValue, address, protection.String())}, nil
}

func (application *app) memoryFree(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory free")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	addressValue := flagSet.String("address", "", "allocation address")
	sizeValue := flagSet.Uint("size", 0, "allocation size in bytes")
	yes := flagSet.Bool("yes", false, "confirm free")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *addressValue == "" {
		return commandResult{}, missingRequired("--address", "Use the exact address returned by 'cecli memory alloc'.")
	}
	address, err := memory.ParseAddress(*addressValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use a valid allocation address.")
	}
	if *sizeValue == 0 || uint64(*sizeValue) > uint64(^uint32(0)) {
		return commandResult{}, usageError("--size must be between 1 and 4294967295", "Use the exact allocation size.")
	}
	preview := map[string]any{"pid": pid, "address": fmt.Sprintf("0x%X", address), "size": *sizeValue, "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nFree %d bytes at 0x%X in PID %d", *sizeValue, address, pid)}, nil
	}
	if err := requireYes(*yes, "memory free", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	freed, err := client.FreeMemory(pid, address, uint32(*sizeValue))
	if err != nil {
		return commandResult{}, err
	}
	if !freed {
		return commandResult{}, operationRejected("memory free", preview)
	}
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("Freed %d bytes at 0x%X", *sizeValue, address)}, nil
}

func (application *app) memoryProtect(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory protect")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	addressValue := flagSet.String("address", "", "base address")
	sizeValue := flagSet.Uint("size", 0, "region size in bytes")
	protectionValue := flagSet.String("protection", "", "noaccess, r, rw, rc, x, rx, or rwx")
	yes := flagSet.Bool("yes", false, "confirm protection change")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *addressValue == "" {
		return commandResult{}, missingRequired("--address", "Use an address from 'cecli memory regions'.")
	}
	address, err := memory.ParseAddress(*addressValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use a valid target address.")
	}
	if *sizeValue == 0 || uint64(*sizeValue) > uint64(^uint32(0)) {
		return commandResult{}, usageError("--size must be between 1 and 4294967295", "Use a bounded region size.")
	}
	if *protectionValue == "" {
		return commandResult{}, missingRequired("--protection", "Use noaccess, r, rw, rc, x, rx, or rwx.")
	}
	protection, err := parsePageProtection(*protectionValue)
	if err != nil {
		return commandResult{}, err
	}
	preview := map[string]any{"pid": pid, "address": fmt.Sprintf("0x%X", address), "size": *sizeValue, "protection": protection.String(), "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nProtect 0x%X + %d as %s", address, *sizeValue, protection.String())}, nil
	}
	if err := requireYes(*yes, "memory protection change", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	change, err := client.ChangeMemoryProtection(pid, address, uint32(*sizeValue), protection)
	if err != nil {
		return commandResult{}, err
	}
	preview["old_protection"] = change.OldPermissions
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("Protection changed: %s -> %s", change.OldPermissions, change.NewPermissions)}, nil
}

func (application *app) memoryRegions(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory regions")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	pagedOnly := flagSet.Bool("paged-only", false, "only paged regions")
	dirtyOnly := flagSet.Bool("dirty-only", false, "only dirty regions")
	noShared := flagSet.Bool("no-shared", false, "exclude shared mappings")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	regions, err := client.Regions(pid, ceserver.NewRegionQueryFlags(*pagedOnly, *dirtyOnly, *noShared))
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{Data: map[string]any{"pid": pid, "regions": regions, "count": len(regions)}, Human: renderRegions(regions)}, nil
}

func (application *app) memoryRead(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory read")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	addressValue := flagSet.String("address", "", "base address in decimal or 0x notation")
	sizeValue := flagSet.Uint("size", 0, "number of bytes to read")
	formatValue := flagSet.String("format", "hex", "hex, base64, or typed")
	typeValue := flagSet.String("type", "", "typed decoder")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *addressValue == "" {
		return commandResult{}, missingRequired("--address", "Use --address 0x7ffdeadbeef.")
	}
	address, err := memory.ParseAddress(*addressValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use a decimal address or 0x-prefixed hexadecimal address.")
	}
	readSize := uint32(*sizeValue)
	if *sizeValue > maximumReadSize {
		return commandResult{}, usageError(fmt.Sprintf("--size exceeds %d bytes", maximumReadSize), "Use a smaller read and stream adjacent ranges separately.")
	}
	if readSize == 0 && *typeValue != "" {
		if inferred, ok := memory.ByteSize(*typeValue); ok {
			readSize = inferred
		}
	}
	if readSize == 0 {
		return commandResult{}, missingRequired("--size", "Provide --size or a fixed-width --type.")
	}
	format := strings.ToLower(*formatValue)
	if format != "hex" && format != "base64" && format != "typed" {
		return commandResult{}, usageError("--format must be hex, base64, or typed", "Use --format hex.")
	}
	if format == "typed" && *typeValue == "" {
		return commandResult{}, missingRequired("--type", "Use --type u32, i32, f32, u64, f64, utf8, or utf16le.")
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	data, err := client.ReadMemory(pid, address, readSize)
	if err != nil {
		return commandResult{}, err
	}
	resultData := map[string]any{
		"pid": pid, "address": fmt.Sprintf("0x%X", address), "requested_size": readSize, "bytes_read": len(data), "format": format,
	}
	switch format {
	case "hex":
		resultData["data"] = strings.ToUpper(hex.EncodeToString(data))
	case "base64":
		resultData["data"] = base64.StdEncoding.EncodeToString(data)
	case "typed":
		decoded, decodeErr := memory.Decode(*typeValue, data)
		if decodeErr != nil {
			return commandResult{}, usageError(decodeErr.Error(), "Increase --size or choose a decoder matching the target data.")
		}
		resultData["type"] = strings.ToLower(*typeValue)
		resultData["data"] = decoded
	}
	human := memory.HexDump(data, address)
	if format == "typed" {
		human += fmt.Sprintf("\n\nDecoded (%s): %v", strings.ToLower(*typeValue), resultData["data"])
	}
	return commandResult{Data: resultData, Human: human}, nil
}

func (application *app) memoryWrite(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory write")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	addressValue := flagSet.String("address", "", "base address in decimal or 0x notation")
	typeValue := flagSet.String("type", "u32", "value encoder")
	value := flagSet.String("value", "", "typed value")
	hexValue := flagSet.String("hex", "", "raw hexadecimal bytes")
	yes := flagSet.Bool("yes", false, "confirm the write")
	verify := flagSet.Bool("verify", false, "read back and compare bytes")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	if *addressValue == "" {
		return commandResult{}, missingRequired("--address", "Use --address 0x7ffdeadbeef.")
	}
	address, err := memory.ParseAddress(*addressValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use a decimal address or 0x-prefixed hexadecimal address.")
	}
	valueWasSet := flagWasSet(flagSet, "value")
	hexWasSet := flagWasSet(flagSet, "hex")
	if valueWasSet == hexWasSet {
		return commandResult{}, usageError("provide exactly one of --value or --hex", "Use --value 100 --type i32, or --hex '64 00 00 00'.")
	}
	var encoded []byte
	if hexWasSet {
		encoded, err = memory.ParseHex(*hexValue)
	} else {
		encoded, err = memory.Encode(*typeValue, *value)
	}
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Check the selected --type and value encoding.")
	}
	if len(encoded) > maximumWriteSize {
		return commandResult{}, usageError(fmt.Sprintf("write exceeds %d bytes", maximumWriteSize), "Split the write into smaller authorized operations.")
	}
	preview := map[string]any{
		"pid": pid, "address": fmt.Sprintf("0x%X", address), "size": len(encoded),
		"bytes_hex": strings.ToUpper(hex.EncodeToString(encoded)), "verify": *verify, "dry_run": application.options.dryRun,
	}
	humanPreview := fmt.Sprintf("PID:     %d\nAddress: 0x%X\nBytes:   %s\nSize:    %d", pid, address, preview["bytes_hex"], len(encoded))
	if application.options.dryRun {
		return commandResult{Data: preview, Human: "DRY RUN\n" + humanPreview}, nil
	}
	if !*yes {
		return commandResult{}, &commandError{
			Code: "CONFIRMATION_REQUIRED", Message: "memory write refused without --yes",
			Suggestion: "Inspect the command with --dry-run, then repeat with --yes and preferably --verify.", ExitCode: 2,
			Details: preview,
		}
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	written, err := client.WriteMemory(pid, address, encoded)
	if err != nil {
		return commandResult{}, err
	}
	preview["written"] = written
	preview["dry_run"] = false
	verified := false
	if *verify {
		readBack, readErr := client.ReadMemory(pid, address, uint32(len(encoded)))
		if readErr != nil {
			return commandResult{}, readErr
		}
		verified = bytes.Equal(readBack, encoded)
		preview["verified"] = verified
		if !verified {
			return commandResult{}, &commandError{
				Code: "WRITE_VERIFICATION_FAILED", Message: "read-back bytes do not match the requested write",
				Suggestion: "Stop further writes and inspect target protections, concurrent mutations, and the selected address.", ExitCode: 30,
				Details: preview,
			}
		}
	}
	human := humanPreview + fmt.Sprintf("\nWritten: %d", written)
	if *verify {
		human += fmt.Sprintf("\nVerified: %t", verified)
	}
	return commandResult{Data: preview, Human: human}, nil
}

func (application *app) memoryScan(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory scan")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	patternValue := flagSet.String("pattern", "", "byte pattern such as '48 8B ?? FF'")
	value := flagSet.String("value", "", "typed exact value")
	typeValue := flagSet.String("type", "i32", "typed value encoder")
	startValue := flagSet.String("start", "0x0", "inclusive scan start")
	endValue := flagSet.String("end", "0x7FFFFFFFFFFF", "exclusive scan end")
	alignment := flagSet.Int("alignment", 1, "address alignment")
	protectionValue := flagSet.String("protection", "readable", "readable, writable, executable, all, or numeric mask")
	limit := flagSet.Int("limit", 1000, "stop after this many matches")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	patternWasSet := flagWasSet(flagSet, "pattern")
	valueWasSet := flagWasSet(flagSet, "value")
	if patternWasSet == valueWasSet {
		return commandResult{}, usageError("provide exactly one of --pattern or --value", "Use --pattern '48 8B ?? FF', or --value 100 --type i32.")
	}
	if *alignment < 1 {
		return commandResult{}, usageError("--alignment must be at least 1", "Use --alignment 1, 2, 4, or 8.")
	}
	if *limit < 1 || *limit > 1_000_000 {
		return commandResult{}, usageError("--limit must be between 1 and 1000000", "Use --limit 1000.")
	}
	start, err := memory.ParseAddress(*startValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use --start with a decimal or 0x-prefixed address.")
	}
	end, err := memory.ParseAddress(*endValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use --end with a decimal or 0x-prefixed address.")
	}
	protection, err := parseProtection(*protectionValue)
	if err != nil {
		return commandResult{}, err
	}
	var pattern []byte
	var mask []byte
	patternDescription := *patternValue
	if patternWasSet {
		pattern, mask, err = memory.ParsePattern(*patternValue)
	} else {
		pattern, err = memory.Encode(*typeValue, *value)
		mask = bytes.Repeat([]byte{'x'}, len(pattern))
		patternDescription = fmt.Sprintf("%s:%s", strings.ToLower(*typeValue), *value)
	}
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Check the pattern tokens or typed value.")
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	addresses, err := client.ScanMemory(application.context, pid, pattern, mask, start, end, *alignment, protection, *limit)
	if err != nil {
		return commandResult{}, err
	}
	hexAddresses := make([]string, len(addresses))
	for index, address := range addresses {
		hexAddresses[index] = fmt.Sprintf("0x%X", address)
	}
	resultData := map[string]any{
		"pid": pid, "pattern": patternDescription, "pattern_hex": strings.ToUpper(hex.EncodeToString(pattern)),
		"start": fmt.Sprintf("0x%X", start), "end": fmt.Sprintf("0x%X", end), "alignment": *alignment,
		"protection": *protectionValue, "matches": hexAddresses, "match_count": len(hexAddresses), "limit_reached": len(hexAddresses) == *limit,
	}
	return commandResult{Data: resultData, Human: renderAddresses(hexAddresses)}, nil
}

func (application *app) memoryAOBScan(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("memory aobscan")
	pidValue := flagSet.Int("pid", 0, "target process ID")
	patternValue := flagSet.String("pattern", "", "byte pattern such as '48 8B ?? FF'")
	value := flagSet.String("value", "", "typed exact value")
	typeValue := flagSet.String("type", "i32", "typed value encoder")
	startValue := flagSet.String("start", "0x0", "inclusive scan start")
	endValue := flagSet.String("end", "0x7FFFFFFFFFFF", "exclusive scan end")
	alignment := flagSet.Int("alignment", 1, "address alignment")
	protectionValue := flagSet.String("protection", "readable", "readable, writable, executable, all, or numeric mask")
	limit := flagSet.Int("limit", 1000, "maximum matches included in command output")
	yes := flagSet.Bool("yes", false, "confirm experimental server-side scanning")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	pid, err := parsePositiveInt32(*pidValue, "--pid")
	if err != nil {
		return commandResult{}, err
	}
	patternWasSet := flagWasSet(flagSet, "pattern")
	valueWasSet := flagWasSet(flagSet, "value")
	if patternWasSet == valueWasSet {
		return commandResult{}, usageError("provide exactly one of --pattern or --value", "Use --pattern '48 8B ?? FF', or --value 100 --type i32.")
	}
	if *alignment < 1 {
		return commandResult{}, usageError("--alignment must be at least 1", "Use --alignment 1, 2, 4, or 8.")
	}
	if *limit < 1 || *limit > 1_000_000 {
		return commandResult{}, usageError("--limit must be between 1 and 1000000", "Use --limit 1000.")
	}
	start, err := memory.ParseAddress(*startValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use --start with a decimal or 0x-prefixed address.")
	}
	end, err := memory.ParseAddress(*endValue)
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Use --end with a decimal or 0x-prefixed address.")
	}
	if end <= start {
		return commandResult{}, usageError("--end must be greater than --start", "Choose a non-empty bounded scan range.")
	}
	protection, err := parseProtection(*protectionValue)
	if err != nil {
		return commandResult{}, err
	}
	var pattern []byte
	var mask []byte
	patternDescription := *patternValue
	if patternWasSet {
		pattern, mask, err = memory.ParsePattern(*patternValue)
	} else {
		pattern, err = memory.Encode(*typeValue, *value)
		mask = bytes.Repeat([]byte{'x'}, len(pattern))
		patternDescription = fmt.Sprintf("%s:%s", strings.ToLower(*typeValue), *value)
	}
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Check the pattern tokens or typed value.")
	}
	preview := map[string]any{
		"pid": pid, "pattern": patternDescription, "pattern_hex": strings.ToUpper(hex.EncodeToString(pattern)), "mask": string(mask),
		"start": fmt.Sprintf("0x%X", start), "end": fmt.Sprintf("0x%X", end), "alignment": *alignment,
		"protection": *protectionValue, "limit": *limit, "backend": "server", "experimental": true, "dry_run": application.options.dryRun,
	}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nExperimental server AOB scan for PID %d from 0x%X to 0x%X", pid, start, end)}, nil
	}
	if err := requireYes(*yes, "experimental server-side AOB scan", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	matches, err := client.ServerAOBScan(pid, pattern, mask, start, end, *alignment, protection)
	if err != nil {
		return commandResult{}, err
	}
	totalMatches := len(matches)
	if len(matches) > *limit {
		matches = matches[:*limit]
	}
	hexAddresses := make([]string, len(matches))
	for index, address := range matches {
		hexAddresses[index] = fmt.Sprintf("0x%X", address)
	}
	preview["matches"] = hexAddresses
	preview["match_count"] = len(hexAddresses)
	preview["total_match_count"] = totalMatches
	preview["limit_reached"] = totalMatches > len(hexAddresses)
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: renderAddresses(hexAddresses)}, nil
}

func (application *app) executeSkills(arguments []string) (commandResult, error) {
	if len(arguments) == 0 || arguments[0] == "list" {
		skills := agentdocs.Skills()
		return commandResult{Data: map[string]any{"skills": skills, "count": len(skills)}, Human: renderDocuments(skills)}, nil
	}
	if len(arguments) != 1 {
		return commandResult{}, usageError("skills accepts at most one name", "Use 'cecli skills' or 'cecli skills memory-inspection'.")
	}
	skill, err := agentdocs.Skill(arguments[0])
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Run 'cecli skills' to list available skills.")
	}
	return commandResult{Data: skill, Human: skill.Content}, nil
}

func (application *app) executeIssue(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("issue requires a subcommand", "Use 'cecli issue create', 'list', 'show', or 'transition'.")
	}
	store, err := feedback.NewStore("")
	if err != nil {
		return commandResult{}, err
	}
	switch arguments[0] {
	case "create":
		flagSet := newFlagSet("issue create")
		issueType := flagSet.String("type", "", "bug, requirement, suggestion, or bad-output")
		message := flagSet.String("message", "", "feedback message")
		contextValue := flagSet.String("context", "", "sanitized JSON object")
		exitCodeValue := flagSet.String("exit-code", "", "related command exit code")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if *issueType == "" {
			return commandResult{}, missingRequired("--type", "Use --type bug, requirement, suggestion, or bad-output.")
		}
		if *message == "" {
			return commandResult{}, missingRequired("--message", "Use --message 'Describe the observed behavior'.")
		}
		var contextData map[string]any
		if *contextValue != "" {
			if err := json.Unmarshal([]byte(*contextValue), &contextData); err != nil {
				return commandResult{}, usageError("--context must be a JSON object", "Use --context '{\"command\":\"memory scan\"}'.")
			}
		}
		exitCode, err := parseOptionalExitCode(*exitCodeValue)
		if err != nil {
			return commandResult{}, err
		}
		issue, err := store.Create(*issueType, *message, Version, contextData, exitCode)
		if err != nil {
			return commandResult{}, usageError(err.Error(), "Use a supported issue type and a non-empty message.")
		}
		return commandResult{Data: issue, Human: fmt.Sprintf("Created %s (%s)", issue.ID, issue.Type)}, nil
	case "list":
		flagSet := newFlagSet("issue list")
		status := flagSet.String("status", "", "filter by status")
		issueType := flagSet.String("type", "", "filter by type")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		issues, err := store.List()
		if err != nil {
			return commandResult{}, err
		}
		filtered := make([]feedback.Issue, 0, len(issues))
		for _, issue := range issues {
			if *status != "" && issue.Status != *status {
				continue
			}
			if *issueType != "" && issue.Type != *issueType {
				continue
			}
			filtered = append(filtered, issue)
		}
		return commandResult{Data: map[string]any{"issues": filtered, "count": len(filtered)}, Human: renderIssues(filtered)}, nil
	case "show":
		if len(arguments) != 2 {
			return commandResult{}, missingRequired("issue ID", "Use 'cecli issue show CECLI-...'.")
		}
		issue, err := store.Show(arguments[1])
		if err != nil {
			return commandResult{}, &commandError{Code: "NOT_FOUND", Message: err.Error(), Suggestion: "Run 'cecli issue list' to find a valid issue ID.", ExitCode: 20}
		}
		return commandResult{Data: issue, Human: renderIssue(issue)}, nil
	case "transition":
		flagSet := newFlagSet("issue transition")
		status := flagSet.String("status", "", "open, in-progress, resolved, or closed")
		if err := parseFlagsAllowArgs(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if *status == "" {
			return commandResult{}, missingRequired("--status", "Use --status open, in-progress, resolved, or closed.")
		}
		if flagSet.NArg() != 1 {
			return commandResult{}, missingRequired("issue ID", "Use 'cecli issue transition --status resolved CECLI-...'.")
		}
		issue, err := store.Transition(flagSet.Arg(0), *status)
		if err != nil {
			return commandResult{}, usageError(err.Error(), "Run 'cecli issue list' and use a supported status.")
		}
		return commandResult{Data: issue, Human: fmt.Sprintf("%s -> %s", issue.ID, issue.Status)}, nil
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown issue subcommand %q", arguments[0]), "Use create, list, show, or transition.")
	}
}

func (application *app) executeSelfCheck(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("self-check")
	checkServer := flagSet.Bool("server", false, "connect to ceserver")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	checks := []map[string]any{
		{"name": "agent_brief", "ok": strings.TrimSpace(agentdocs.Brief()) != ""},
		{"name": "agent_rules", "ok": len(agentdocs.Rules()) >= 3},
		{"name": "agent_skills", "ok": len(agentdocs.Skills()) >= 3},
	}
	catalogProblems := validateHelpCatalog()
	catalogCheck := map[string]any{"name": "command_catalog", "ok": len(catalogProblems) == 0}
	if len(catalogProblems) > 0 {
		catalogCheck["errors"] = catalogProblems
	}
	checks = append(checks, catalogCheck)
	if _, _, err := net.SplitHostPort(application.options.endpoint); err != nil {
		checks = append(checks, map[string]any{"name": "endpoint", "ok": false, "error": err.Error()})
	} else {
		checks = append(checks, map[string]any{"name": "endpoint", "ok": true})
	}
	if _, err := feedback.NewStore(""); err != nil {
		checks = append(checks, map[string]any{"name": "issue_store", "ok": false, "error": err.Error()})
	} else {
		checks = append(checks, map[string]any{"name": "issue_store", "ok": true})
	}
	if *checkServer {
		client, err := application.dial()
		if err != nil {
			checks = append(checks, map[string]any{"name": "ceserver", "ok": false, "error": err.Error()})
		} else {
			_, infoErr := client.ServerInfo()
			client.Close()
			checks = append(checks, map[string]any{"name": "ceserver", "ok": infoErr == nil, "error": errorString(infoErr)})
		}
	}
	allPassed := true
	for _, check := range checks {
		if passed, _ := check["ok"].(bool); !passed {
			allPassed = false
		}
	}
	if !allPassed {
		return commandResult{}, &commandError{Code: "SELF_CHECK_FAILED", Message: "one or more self-checks failed", Suggestion: "Inspect the check details, correct the environment, and run self-check again.", ExitCode: 1, Details: map[string]any{"checks": checks}}
	}
	return commandResult{Data: map[string]any{"ok": true, "checks": checks}, Human: renderChecks(checks)}, nil
}

func (application *app) dial() (*ceserver.Client, error) {
	client, err := ceserver.Dial(application.context, application.options.endpoint, application.options.timeout)
	if err != nil {
		return nil, err
	}
	if application.options.connectionName != "" {
		if err := client.SetConnectionName(application.options.connectionName); err != nil {
			client.Close()
			return nil, err
		}
	}
	return client, nil
}

func newFlagSet(name string) *flag.FlagSet {
	flagSet := flag.NewFlagSet(name, flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	return flagSet
}

func parseFlags(flagSet *flag.FlagSet, arguments []string) error {
	if err := parseFlagsAllowArgs(flagSet, arguments); err != nil {
		return err
	}
	if flagSet.NArg() > 0 {
		return usageError("unexpected arguments: "+strings.Join(flagSet.Args(), " "), "Remove positional arguments not shown in the command usage.")
	}
	return nil
}

func parseFlagsAllowArgs(flagSet *flag.FlagSet, arguments []string) error {
	if err := flagSet.Parse(arguments); err != nil {
		return usageError(err.Error(), "Run 'cecli help' for command usage.")
	}
	return nil
}

func flagWasSet(flagSet *flag.FlagSet, name string) bool {
	found := false
	flagSet.Visit(func(visited *flag.Flag) {
		if visited.Name == name {
			found = true
		}
	})
	return found
}

func parseProtection(value string) (ceserver.Protection, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "readable", "r":
		return ceserver.ProtectionReadable, nil
	case "writable", "w":
		return ceserver.ProtectionReadWrite | ceserver.ProtectionWriteCopy | ceserver.ProtectionExecuteReadWrite, nil
	case "executable", "x":
		return ceserver.ProtectionExecute | ceserver.ProtectionExecuteRead | ceserver.ProtectionExecuteReadWrite, nil
	case "all":
		return ceserver.ProtectionNoAccess | ceserver.ProtectionReadable | ceserver.ProtectionExecute, nil
	default:
		parsed, err := strconv.ParseUint(value, 0, 32)
		if err != nil {
			return 0, usageError("invalid --protection value", "Use readable, writable, executable, all, or a numeric mask.")
		}
		return ceserver.Protection(parsed), nil
	}
}

func renderProcesses(processes []ceserver.Process) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "PID\tNAME")
	for _, process := range processes {
		fmt.Fprintf(writer, "%d\t%s\n", process.PID, process.Name)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}

func renderModules(modules []ceserver.Module) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "BASE\tSIZE\tOFFSET\tNAME")
	for _, module := range modules {
		fmt.Fprintf(writer, "0x%X\t0x%X\t0x%X\t%s\n", module.BaseAddress, module.Size, module.FileOffset, module.Name)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}

func renderThreads(threads []ceserver.Thread) string {
	lines := make([]string, len(threads)+1)
	lines[0] = "TID"
	for index, thread := range threads {
		lines[index+1] = strconv.FormatInt(int64(thread.TID), 10)
	}
	return strings.Join(lines, "\n")
}

func renderRegions(regions []ceserver.Region) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "BASE\tEND\tSIZE\tPERM\tTYPE")
	for _, region := range regions {
		fmt.Fprintf(writer, "0x%X\t0x%X\t0x%X\t%s\t%s\n", region.BaseAddress, region.BaseAddress+region.Size, region.Size, region.Permissions, region.TypeName)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}

func renderAddresses(addresses []string) string {
	if len(addresses) == 0 {
		return "No matches."
	}
	return strings.Join(addresses, "\n")
}

func renderDocuments(documents []agentdocs.Document) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tDESCRIPTION")
	for _, document := range documents {
		fmt.Fprintf(writer, "%s\t%s\n", document.Name, document.Description)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}

func renderIssues(issues []feedback.Issue) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tTYPE\tSTATUS\tMESSAGE")
	for _, issue := range issues {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", issue.ID, issue.Type, issue.Status, firstLine(issue.Message))
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}

func renderIssue(issue feedback.Issue) string {
	return fmt.Sprintf("ID:      %s\nType:    %s\nStatus:  %s\nCreated: %s\nMessage: %s", issue.ID, issue.Type, issue.Status, issue.CreatedAt.Format("2006-01-02T15:04:05Z"), issue.Message)
}

func renderChecks(checks []map[string]any) string {
	lines := make([]string, 0, len(checks))
	for _, check := range checks {
		status := "PASS"
		if passed, _ := check["ok"].(bool); !passed {
			status = "FAIL"
		}
		lines = append(lines, fmt.Sprintf("%-4s %s", status, check["name"]))
	}
	return strings.Join(lines, "\n")
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return line
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
