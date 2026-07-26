package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
	"github.com/chengyixu/cheat-engine-cli/internal/memory"
)

func (application *app) executeSymbol(arguments []string) (commandResult, error) {
	if len(arguments) == 0 || arguments[0] != "list" {
		return commandResult{}, usageError("symbol requires the 'list' subcommand", "Use 'cecli symbol list --path /remote/module.so'.")
	}
	flagSet := newFlagSet("symbol list")
	path := flagSet.String("path", "", "remote ELF path")
	fileOffsetValue := flagSet.String("file-offset", "0", "ELF offset inside the remote file")
	moduleBaseValue := flagSet.String("module-base", "0", "base to add to non-executable symbol addresses")
	filterValue := flagSet.String("filter", "", "case-insensitive symbol name filter")
	limit := flagSet.Int("limit", 10000, "maximum symbols returned")
	if err := parseFlags(flagSet, arguments[1:]); err != nil {
		return commandResult{}, err
	}
	if err := validateRemotePath(*path); err != nil {
		return commandResult{}, err
	}
	fileOffset, err := memory.ParseAddress(*fileOffsetValue)
	if err != nil || fileOffset > uint64(^uint32(0)) {
		return commandResult{}, usageError("invalid --file-offset", "Use a 32-bit decimal or 0x-prefixed file offset.")
	}
	moduleBase, err := memory.ParseAddress(*moduleBaseValue)
	if err != nil {
		return commandResult{}, usageError("invalid --module-base", "Use 0 or a valid mapped module base address.")
	}
	if *limit < 1 || *limit > 1_000_000 {
		return commandResult{}, usageError("--limit must be between 1 and 1000000", "Use --limit 10000.")
	}
	client, err := application.dial()
	if err != nil {
		return commandResult{}, err
	}
	defer client.Close()
	symbolList, err := client.Symbols(*path, uint32(fileOffset))
	if err != nil {
		return commandResult{}, err
	}
	filtered := make([]ceserver.Symbol, 0, min(len(symbolList.Symbols), *limit))
	for _, symbol := range symbolList.Symbols {
		if *filterValue != "" && !strings.Contains(strings.ToLower(symbol.Name), strings.ToLower(*filterValue)) {
			continue
		}
		if !symbolList.Executable && moduleBase != 0 {
			symbol.Address += moduleBase
		}
		filtered = append(filtered, symbol)
		if len(filtered) >= *limit {
			break
		}
	}
	data := map[string]any{
		"path": *path, "file_offset": fmt.Sprintf("0x%X", fileOffset), "module_base": fmt.Sprintf("0x%X", moduleBase),
		"executable": symbolList.Executable, "symbols": filtered, "count": len(filtered),
		"total_symbols": symbolList.SymbolCount, "limit_reached": len(filtered) == *limit,
	}
	return commandResult{Data: data, Human: renderSymbols(filtered)}, nil
}

func renderSymbols(symbols []ceserver.Symbol) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "ADDRESS\tSIZE\tTYPE\tNAME")
	for _, symbol := range symbols {
		fmt.Fprintf(writer, "0x%X\t%d\t%d\t%s\n", symbol.Address, symbol.Size, symbol.Type, symbol.Name)
	}
	writer.Flush()
	return strings.TrimRight(builder.String(), "\n")
}
