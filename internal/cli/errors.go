package cli

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
)

type commandError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Suggestion string         `json:"suggestion"`
	ExitCode   int            `json:"-"`
	Details    map[string]any `json:"details,omitempty"`
}

func (commandError *commandError) Error() string {
	return commandError.Message
}

func usageError(message, suggestion string) *commandError {
	return &commandError{Code: "INVALID_USAGE", Message: message, Suggestion: suggestion, ExitCode: 2}
}

func missingRequired(name, example string) *commandError {
	return &commandError{
		Code: "MISSING_REQUIRED", Message: fmt.Sprintf("missing required %s", name),
		Suggestion: example, ExitCode: 2,
	}
}

func normalizeError(err error) *commandError {
	if err == nil {
		return nil
	}
	var existing *commandError
	if errors.As(err, &existing) {
		return existing
	}
	if errors.Is(err, context.Canceled) {
		return &commandError{Code: "INTERRUPTED", Message: "operation cancelled", Suggestion: "Run the command again when ready.", ExitCode: 130}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &commandError{Code: "TIMEOUT", Message: "operation timed out", Suggestion: "Increase --timeout or reduce the scan/read range.", ExitCode: 1}
	}
	var networkError *net.OpError
	if errors.As(err, &networkError) {
		return &commandError{
			Code: "SERVER_UNREACHABLE", Message: err.Error(),
			Suggestion: "Start ceserver and verify --endpoint host:port and network access.", ExitCode: 10,
		}
	}
	var protocolError *ceserver.ProtocolError
	if errors.As(err, &protocolError) {
		return &commandError{
			Code: "CESERVER_PROTOCOL_ERROR", Message: protocolError.Error(),
			Suggestion: "Verify the ceserver version, target permissions, PID, and requested address range.", ExitCode: 1,
		}
	}
	return &commandError{Code: "OPERATION_FAILED", Message: err.Error(), Suggestion: "Run with corrected inputs or record the failure with 'cecli issue create'.", ExitCode: 1}
}
