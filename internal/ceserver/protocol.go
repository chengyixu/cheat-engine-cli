package ceserver

const (
	commandGetVersion                 byte   = 0
	commandCloseConnection            byte   = 1
	commandTerminateServer            byte   = 2
	commandOpenProcess                byte   = 3
	commandCreateToolhelp32Snapshot   byte   = 4
	commandProcess32First             byte   = 5
	commandProcess32Next              byte   = 6
	commandCloseHandle                byte   = 7
	commandReadProcessMemory          byte   = 9
	commandWriteProcessMemory         byte   = 10
	commandStartDebug                 byte   = 11
	commandWaitForDebugEvent          byte   = 13
	commandContinueFromDebugEvent     byte   = 14
	commandSetBreakpoint              byte   = 15
	commandRemoveBreakpoint           byte   = 16
	commandSuspendThread              byte   = 17
	commandResumeThread               byte   = 18
	commandGetThreadContext           byte   = 19
	commandSetThreadContext           byte   = 20
	commandGetArchitecture            byte   = 21
	commandGetSymbols                 byte   = 24
	commandLoadExtension              byte   = 25
	commandAllocateMemory             byte   = 26
	commandFreeMemory                 byte   = 27
	commandCreateRemoteThread         byte   = 28
	commandLoadModule                 byte   = 29
	commandSetSpeed                   byte   = 30
	commandCreateToolhelp32SnapshotEx byte   = 35
	commandVirtualQueryExFull         byte   = 31
	commandGetRegionInfo              byte   = 32
	commandGetABI                     byte   = 33
	commandSetConnectionName          byte   = 34
	commandChangeMemoryProtection     byte   = 36
	commandGetOptions                 byte   = 37
	commandGetOptionValue             byte   = 38
	commandSetOptionValue             byte   = 39
	commandOpenNamedPipe              byte   = 41
	commandPipeRead                   byte   = 42
	commandPipeWrite                  byte   = 43
	commandGetServerPath              byte   = 44
	commandIsAndroid                  byte   = 45
	commandLoadModuleEx               byte   = 46
	commandSetCurrentPath             byte   = 47
	commandGetCurrentPath             byte   = 48
	commandEnumerateFiles             byte   = 49
	commandGetFilePermissions         byte   = 50
	commandSetFilePermissions         byte   = 51
	commandGetFile                    byte   = 52
	commandPutFile                    byte   = 53
	commandCreateDirectory            byte   = 54
	commandDeleteFile                 byte   = 55
	commandAOBScan                    byte   = 200
	toolhelpSnapshotProcess           uint32 = 0x2
	toolhelpSnapshotThread            uint32 = 0x4
	toolhelpSnapshotModule            uint32 = 0x8
	virtualQueryPagedOnly             byte   = 1
	virtualQueryDirtyOnly             byte   = 2
	virtualQueryNoShared              byte   = 4
	pageNoAccess                      uint32 = 1
	pageReadOnly                      uint32 = 2
	pageReadWrite                     uint32 = 4
	pageWriteCopy                     uint32 = 8
	pageExecute                       uint32 = 16
	pageExecuteRead                   uint32 = 32
	pageExecuteReadWrite              uint32 = 64
	memoryMapped                      uint32 = 262144
	memoryPrivate                     uint32 = 131072
	maximumProtocolStringLength              = 1 << 20
	maximumCollectionEntries                 = 5_000_000
	maximumRemoteFileSize                    = 256 << 20
	maximumThreadContextSize                 = 1 << 20
	maximumServerAOBPatternSize              = 1 << 20
)

type Architecture uint8

const (
	ArchitectureX86 Architecture = iota
	ArchitectureX8664
	ArchitectureARM
	ArchitectureARM64
)

func (architecture Architecture) String() string {
	switch architecture {
	case ArchitectureX86:
		return "x86"
	case ArchitectureX8664:
		return "x86_64"
	case ArchitectureARM:
		return "arm"
	case ArchitectureARM64:
		return "arm64"
	default:
		return "unknown"
	}
}

type ABI uint8

func (abi ABI) String() string {
	if abi == 0 {
		return "windows"
	}
	if abi == 1 {
		return "unix"
	}
	return "unknown"
}

type RegionQueryFlags uint8

func NewRegionQueryFlags(pagedOnly, dirtyOnly, noShared bool) RegionQueryFlags {
	var flags byte
	if pagedOnly {
		flags |= virtualQueryPagedOnly
	}
	if dirtyOnly {
		flags |= virtualQueryDirtyOnly
	}
	if noShared {
		flags |= virtualQueryNoShared
	}
	return RegionQueryFlags(flags)
}

type Protection uint32

const (
	ProtectionNoAccess         Protection = Protection(pageNoAccess)
	ProtectionReadOnly         Protection = Protection(pageReadOnly)
	ProtectionReadWrite        Protection = Protection(pageReadWrite)
	ProtectionWriteCopy        Protection = Protection(pageWriteCopy)
	ProtectionExecute          Protection = Protection(pageExecute)
	ProtectionExecuteRead      Protection = Protection(pageExecuteRead)
	ProtectionExecuteReadWrite Protection = Protection(pageExecuteReadWrite)
	ProtectionReadable                    = ProtectionReadOnly | ProtectionReadWrite | ProtectionWriteCopy | ProtectionExecuteRead | ProtectionExecuteReadWrite
)

func (protection Protection) String() string {
	switch protection {
	case ProtectionNoAccess:
		return "---"
	case ProtectionReadOnly:
		return "r--"
	case ProtectionReadWrite:
		return "rw-"
	case ProtectionWriteCopy:
		return "rc-"
	case ProtectionExecute:
		return "--x"
	case ProtectionExecuteRead:
		return "r-x"
	case ProtectionExecuteReadWrite:
		return "rwx"
	default:
		return "unknown"
	}
}

type MemoryType uint32

const (
	MemoryTypePrivate MemoryType = MemoryType(memoryPrivate)
	MemoryTypeMapped  MemoryType = MemoryType(memoryMapped)
)

func (memoryType MemoryType) String() string {
	switch memoryType {
	case MemoryTypePrivate:
		return "private"
	case MemoryTypeMapped:
		return "mapped"
	default:
		return "unknown"
	}
}
