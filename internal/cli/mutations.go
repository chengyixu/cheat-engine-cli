package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
)

func requireYes(confirmed bool, operation string, details map[string]any) error {
	if confirmed {
		return nil
	}
	return &commandError{
		Code: "CONFIRMATION_REQUIRED", Message: operation + " refused without --yes",
		Suggestion: "Inspect the command with --dry-run, then repeat with --yes only when the target and mutation are authorized.",
		ExitCode:   2, Details: details,
	}
}

func operationRejected(operation string, details map[string]any) error {
	return &commandError{
		Code: "OPERATION_REJECTED", Message: "ceserver rejected the " + operation,
		Suggestion: "Verify target permissions, extension availability, path/address validity, and current server state.",
		ExitCode:   30, Details: details,
	}
}

func validateRemotePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return missingRequired("remote path", "Provide a non-empty target path.")
	}
	if strings.ContainsRune(path, 0) {
		return usageError("remote path contains a NUL byte", "Use a normal filesystem path.")
	}
	if len(path) > int(^uint16(0)) {
		return usageError("remote path exceeds 65535 bytes", "Use a shorter target path.")
	}
	return nil
}

func parsePageProtection(value string) (ceserver.Protection, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "noaccess", "none", "---":
		return ceserver.ProtectionNoAccess, nil
	case "r", "r--", "readonly":
		return ceserver.ProtectionReadOnly, nil
	case "rw", "rw-", "readwrite":
		return ceserver.ProtectionReadWrite, nil
	case "rc", "rc-", "writecopy":
		return ceserver.ProtectionWriteCopy, nil
	case "x", "--x", "execute":
		return ceserver.ProtectionExecute, nil
	case "rx", "r-x", "executeread":
		return ceserver.ProtectionExecuteRead, nil
	case "rwx", "executereadwrite":
		return ceserver.ProtectionExecuteReadWrite, nil
	}
	parsed, err := strconv.ParseUint(normalized, 0, 32)
	if err != nil {
		return 0, usageError("invalid page protection", "Use noaccess, r, rw, rc, x, rx, rwx, or the corresponding numeric code.")
	}
	protection := ceserver.Protection(parsed)
	if protection.String() == "unknown" {
		return 0, usageError(fmt.Sprintf("unsupported page protection code %d", parsed), "Use one exact Cheat Engine page protection code, not a combined scan mask.")
	}
	return protection, nil
}

func parseRemoteMode(value string) (uint32, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return 0, missingRequired("--mode", "Use an octal mode such as --mode 0755.")
	}
	base := 8
	if strings.HasPrefix(normalized, "0o") || strings.HasPrefix(normalized, "0O") {
		normalized = normalized[2:]
	}
	parsed, err := strconv.ParseUint(normalized, base, 32)
	if err != nil || parsed > 0o7777 {
		return 0, usageError("invalid --mode", "Use an octal mode between 0000 and 7777, such as 0755.")
	}
	return uint32(parsed), nil
}

func parseHandle(value string) (uint32, error) {
	if strings.TrimSpace(value) == "" {
		return 0, missingRequired("--handle", "Provide the numeric handle returned by cecli.")
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 0, 32)
	if err != nil || parsed == 0 {
		return 0, usageError("invalid --handle", "Use a positive decimal or 0x-prefixed 32-bit handle.")
	}
	return uint32(parsed), nil
}
