package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
)

func (application *app) executeRemote(arguments []string) (commandResult, error) {
	if len(arguments) == 0 {
		return commandResult{}, usageError("remote requires a subcommand", "Use ls, stat, get, put, mkdir, rm, chmod, pwd, or cd.")
	}
	switch arguments[0] {
	case "pwd":
		if len(arguments) != 1 {
			return commandResult{}, usageError("remote pwd accepts no arguments", "Use 'cecli remote pwd'.")
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		path, err := client.CurrentPath()
		if err != nil {
			return commandResult{}, err
		}
		return commandResult{Data: map[string]string{"path": path}, Human: path}, nil
	case "cd":
		flagSet := newFlagSet("remote cd")
		path := flagSet.String("path", "", "new ceserver current directory")
		yes := flagSet.Bool("yes", false, "confirm server state change")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if err := validateRemotePath(*path); err != nil {
			return commandResult{}, err
		}
		preview := map[string]any{"path": *path, "dry_run": application.options.dryRun}
		if application.options.dryRun {
			return commandResult{Data: preview, Human: "DRY RUN\nRemote cwd -> " + *path}, nil
		}
		if err := requireYes(*yes, "remote current-directory change", preview); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		changed, err := client.SetCurrentPath(*path)
		if err != nil {
			return commandResult{}, err
		}
		if !changed {
			return commandResult{}, operationRejected("remote current-directory change", preview)
		}
		preview["dry_run"] = false
		return commandResult{Data: preview, Human: "Remote cwd -> " + *path}, nil
	case "ls":
		flagSet := newFlagSet("remote ls")
		path := flagSet.String("path", ".", "remote directory path")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if err := validateRemotePath(*path); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		files, err := client.ListRemoteFiles(*path)
		if err != nil {
			return commandResult{}, err
		}
		return commandResult{Data: map[string]any{"path": *path, "files": files, "count": len(files)}, Human: renderRemoteFiles(files)}, nil
	case "stat":
		flagSet := newFlagSet("remote stat")
		path := flagSet.String("path", "", "remote path")
		if err := parseFlags(flagSet, arguments[1:]); err != nil {
			return commandResult{}, err
		}
		if err := validateRemotePath(*path); err != nil {
			return commandResult{}, err
		}
		client, err := application.dial()
		if err != nil {
			return commandResult{}, err
		}
		defer client.Close()
		permissions, err := client.RemoteFilePermissions(*path)
		if err != nil {
			return commandResult{}, err
		}
		data := map[string]any{"path": *path, "permissions": permissions, "mode": fmt.Sprintf("%04o", permissions)}
		return commandResult{Data: data, Human: fmt.Sprintf("%s %04o", *path, permissions)}, nil
	case "get":
		return application.remoteGet(arguments[1:])
	case "put":
		return application.remotePut(arguments[1:])
	case "mkdir":
		return application.remotePathMutation("mkdir", arguments[1:])
	case "rm":
		return application.remotePathMutation("rm", arguments[1:])
	case "chmod":
		return application.remoteChmod(arguments[1:])
	default:
		return commandResult{}, usageError(fmt.Sprintf("unknown remote subcommand %q", arguments[0]), "Use ls, stat, get, put, mkdir, rm, chmod, pwd, or cd.")
	}
}

func (application *app) remoteGet(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("remote get")
	remotePath := flagSet.String("remote", "", "remote source path")
	localPath := flagSet.String("local", "", "local destination path")
	force := flagSet.Bool("force", false, "overwrite an existing local file")
	yes := flagSet.Bool("yes", false, "confirm local file write")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	if err := validateRemotePath(*remotePath); err != nil {
		return commandResult{}, err
	}
	if strings.TrimSpace(*localPath) == "" || strings.ContainsRune(*localPath, 0) {
		return commandResult{}, missingRequired("--local", "Provide a valid local destination path.")
	}
	if info, err := os.Stat(*localPath); err == nil {
		if info.IsDir() {
			return commandResult{}, usageError("--local points to a directory", "Provide a destination file path.")
		}
		if !*force {
			return commandResult{}, &commandError{Code: "LOCAL_FILE_EXISTS", Message: fmt.Sprintf("local file %q already exists", *localPath), Suggestion: "Choose another destination or add --force to overwrite it.", ExitCode: 30}
		}
	} else if !os.IsNotExist(err) {
		return commandResult{}, err
	}
	preview := map[string]any{"remote": *remotePath, "local": *localPath, "force": *force, "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\n%s -> %s", *remotePath, *localPath)}, nil
	}
	if err := requireYes(*yes, "remote file download", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	content, err := client.GetRemoteFile(*remotePath)
	if err != nil {
		return commandResult{}, err
	}
	if err := os.WriteFile(*localPath, content, 0o600); err != nil {
		return commandResult{}, fmt.Errorf("write local file: %w", err)
	}
	preview["size"] = len(content)
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("Downloaded %d bytes: %s -> %s", len(content), *remotePath, *localPath)}, nil
}

func (application *app) remotePut(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("remote put")
	localPath := flagSet.String("local", "", "local source path")
	remotePath := flagSet.String("remote", "", "remote destination path")
	yes := flagSet.Bool("yes", false, "confirm remote file write")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	if strings.TrimSpace(*localPath) == "" || strings.ContainsRune(*localPath, 0) {
		return commandResult{}, missingRequired("--local", "Provide a valid local source file.")
	}
	if err := validateRemotePath(*remotePath); err != nil {
		return commandResult{}, err
	}
	info, err := os.Stat(*localPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("inspect local file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return commandResult{}, usageError("--local must be a regular file", "Provide a regular file smaller than 256 MiB.")
	}
	if info.Size() > maximumTransferSize {
		return commandResult{}, usageError("local file exceeds 256 MiB transfer limit", "Use a smaller file or another authenticated transfer mechanism.")
	}
	preview := map[string]any{"local": *localPath, "remote": *remotePath, "size": info.Size(), "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\n%s -> %s (%d bytes)", *localPath, *remotePath, info.Size())}, nil
	}
	if err := requireYes(*yes, "remote file write", preview); err != nil {
		return commandResult{}, err
	}
	content, err := os.ReadFile(*localPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("read local file: %w", err)
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	written, err := client.PutRemoteFile(*remotePath, content)
	if err != nil {
		return commandResult{}, err
	}
	if !written {
		return commandResult{}, operationRejected("remote file write", preview)
	}
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("Uploaded %d bytes: %s -> %s", len(content), *localPath, *remotePath)}, nil
}

func (application *app) remotePathMutation(operation string, arguments []string) (commandResult, error) {
	flagSet := newFlagSet("remote " + operation)
	path := flagSet.String("path", "", "remote path")
	yes := flagSet.Bool("yes", false, "confirm remote filesystem mutation")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	if err := validateRemotePath(*path); err != nil {
		return commandResult{}, err
	}
	preview := map[string]any{"operation": operation, "path": *path, "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\n%s %s", operation, *path)}, nil
	}
	if err := requireYes(*yes, "remote "+operation, preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	var changed bool
	if operation == "mkdir" {
		changed, err = client.CreateRemoteDirectory(*path)
	} else {
		changed, err = client.DeleteRemotePath(*path)
	}
	if err != nil {
		return commandResult{}, err
	}
	if !changed {
		return commandResult{}, operationRejected("remote "+operation, preview)
	}
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("%s %s", operation, *path)}, nil
}

func (application *app) remoteChmod(arguments []string) (commandResult, error) {
	flagSet := newFlagSet("remote chmod")
	path := flagSet.String("path", "", "remote path")
	modeValue := flagSet.String("mode", "", "octal mode such as 0755")
	yes := flagSet.Bool("yes", false, "confirm permission change")
	if err := parseFlags(flagSet, arguments); err != nil {
		return commandResult{}, err
	}
	if err := validateRemotePath(*path); err != nil {
		return commandResult{}, err
	}
	mode, err := parseRemoteMode(*modeValue)
	if err != nil {
		return commandResult{}, err
	}
	preview := map[string]any{"path": *path, "mode": fmt.Sprintf("%04o", mode), "dry_run": application.options.dryRun}
	if application.options.dryRun {
		return commandResult{Data: preview, Human: fmt.Sprintf("DRY RUN\nchmod %04o %s", mode, *path)}, nil
	}
	if err := requireYes(*yes, "remote permission change", preview); err != nil {
		return commandResult{}, err
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	changed, err := client.SetRemoteFilePermissions(*path, mode)
	if err != nil {
		return commandResult{}, err
	}
	if !changed {
		return commandResult{}, operationRejected("remote permission change", preview)
	}
	preview["dry_run"] = false
	return commandResult{Data: preview, Human: fmt.Sprintf("chmod %04o %s", mode, *path)}, nil
}

func renderRemoteFiles(files []ceserver.RemoteFile) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "TYPE\tNAME")
	for _, file := range files {
		fmt.Fprintf(writer, "%s\t%s\n", file.Type, file.Name)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}

func renderServerOptions(options []ceserver.ServerOption) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tTYPE\tVALUE\tDESCRIPTION")
	for _, option := range options {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", option.Name, option.Type, option.CurrentValue, option.Description)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}
