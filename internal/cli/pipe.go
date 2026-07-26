package cli

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/chengyixu/cheat-engine-cli/internal/memory"
)

func (application *app) executePipe(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("pipe requires a subcommand", "Use open, read, write, or close.")
	}
	switch arguments[0] {
	case "open":
		flagSet := newFlagSet("pipe open")
		name := flagSet.String("name", "", "named pipe name")
		timeout := flagSet.Uint("timeout-ms", 5000, "open timeout in milliseconds")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if strings.TrimSpace(*name) == "" || strings.ContainsRune(*name, 0) {
			return commandResult{}, missingRequired("--name", "Provide a valid ceserver pipe name.")
		}
		if uint64(*timeout) > uint64(^uint32(0)) {
			return commandResult{}, usageError("--timeout-ms exceeds uint32", "Use a smaller timeout.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		handle, err := client.OpenPipe(*name, uint32(*timeout))
		if err != nil {
			return commandResult{}, err
		}
		data := map[string]any{"name": *name, "handle": handle, "handle_hex": fmt.Sprintf("0x%X", handle), "timeout_ms": *timeout}
		return commandResult{Data: data, Human: fmt.Sprintf("Opened %s as handle %d (0x%X)", *name, handle, handle)}, nil
	case "read":
		flagSet := newFlagSet("pipe read")
		handleValue := flagSet.String("handle", "", "pipe handle")
		size := flagSet.Uint("size", 0, "maximum bytes to read")
		timeout := flagSet.Uint("timeout-ms", 5000, "read timeout in milliseconds")
		format := flagSet.String("format", "hex", "hex or base64")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		handle, err := parseHandle(*handleValue)
		if err != nil {
			return commandResult{}, err
		}
		if *size == 0 || *size > maximumReadSize {
			return commandResult{}, usageError(fmt.Sprintf("--size must be between 1 and %d", maximumReadSize), "Use a bounded pipe read size.")
		}
		if uint64(*timeout) > uint64(^uint32(0)) {
			return commandResult{}, usageError("--timeout-ms exceeds uint32", "Use a smaller timeout.")
		}
		if *format != "hex" && *format != "base64" {
			return commandResult{}, usageError("--format must be hex or base64", "Use --format hex.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		data, err := client.ReadPipe(handle, uint32(*size), uint32(*timeout))
		if err != nil {
			return commandResult{}, err
		}
		encoded := strings.ToUpper(hex.EncodeToString(data))
		if *format == "base64" {
			encoded = base64.StdEncoding.EncodeToString(data)
		}
		result := map[string]any{"handle": handle, "bytes_read": len(data), "format": *format, "data": encoded}
		return commandResult{Data: result, Human: memory.HexDump(data, 0)}, nil
	case "write":
		return application.pipeWrite(arguments[1:])
	case "close":
		flagSet := newFlagSet("pipe close")
		handleValue := flagSet.String("handle", "", "pipe handle")
		yes := flagSet.Bool("yes", false, "confirm pipe close")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		handle, err := parseHandle(*handleValue)
		if err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"handle": handle, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nClose pipe handle %d", handle)}, nil
		}
		if err := requireYes(*yes, "pipe handle close", preview); err != nil {
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
		return commandResult{Data: preview, Human: fmt.Sprintf("Closed pipe handle %d", handle)}, nil
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown pipe subcommand %q", arguments[0]), "Use open, read, write, or close.")
	}
}

func (application *app) pipeWrite(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("pipe write")
	handleValue := flagSet.String("handle", "", "pipe handle")
	hexValue := flagSet.String("hex", "", "hexadecimal bytes")
	textValue := flagSet.String("text", "", "UTF-8 text")
	fileValue := flagSet.String("file", "", "local file content")
	timeout := flagSet.Uint("timeout-ms", 5000, "write timeout in milliseconds")
	yes := flagSet.Bool("yes", false, "confirm pipe write")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	handle, err := parseHandle(*handleValue)
	if err != nil {
		return commandResult{}, err
	}
	selected := 0
	for _, name := range []string{"hex", "text", "file"} {
		if flagWasSet(flagSet, name) {
			selected++
		}
	}
	if selected != 1 {
		return commandResult{}, usageError("provide exactly one of --hex, --text, or --file", "Use --text 'command', --hex '01 FF', or --file payload.bin.")
	}
	var data []byte
	if flagWasSet(flagSet, "hex") {
		data, err = memory.ParseHex(*hexValue)
	} else if flagWasSet(flagSet, "text") {
		data = []byte(*textValue)
	} else {
		data, err = os.ReadFile(*fileValue)
	}
	if err != nil {
		return commandResult{}, usageError(err.Error(), "Check the selected pipe input.")
	}
	if len(data) > maximumWriteSize {
		return commandResult{}, usageError(fmt.Sprintf("pipe write exceeds %d bytes", maximumWriteSize), "Split the message into smaller protocol frames.")
	}
	if uint64(*timeout) > uint64(^uint32(0)) {
		return commandResult{}, usageError("--timeout-ms exceeds uint32", "Use a smaller timeout.")
	}
	preview := map[string]any{"handle": handle, "size": len(data), "bytes_hex": strings.ToUpper(hex.EncodeToString(data)), "timeout_ms": *timeout, "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nWrite %d bytes to pipe handle %d", len(data), handle)}, nil
	}
	if err := requireYes(*yes, "named-pipe write", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	written, err := client.WritePipe(handle, data, uint32(*timeout))
	if err != nil {
		return commandResult{}, err
	}
	if written != int32(len(data)) {
		preview["written"] = written
		return commandResult{}, operationRejected("named-pipe write", preview)
	}
	preview["written"] = written
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("Wrote %d bytes to pipe handle %d", written, handle)}, nil
}
