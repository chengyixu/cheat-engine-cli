//go:build windows

package localbridge

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	processVMOperation           = 0x0008
	processVMRead                = 0x0010
	processVMWrite               = 0x0020
	processQueryInformation      = 0x0400
	th32csSnapshotProcess        = 0x00000002
	invalidHandleValue           = ^uintptr(0)
	maximumPath                  = 260
	memCommit                    = 0x1000
	memPrivate                   = 0x20000
	memMapped                    = 0x40000
	memImage                     = 0x1000000
	pageNoAccess                 = 0x01
	pageReadOnly                 = 0x02
	pageReadWrite                = 0x04
	pageWriteCopy                = 0x08
	pageExecute                  = 0x10
	pageExecuteRead              = 0x20
	pageExecuteReadWrite         = 0x40
	pageExecuteWriteCopy         = 0x80
	imageFileMachineUnknown      = 0x0000
	imageFileMachineI386         = 0x014c
	imageFileMachineARMNT        = 0x01c4
	imageFileMachineAMD64        = 0x8664
	imageFileMachineARM64        = 0xaa64
	architectureX86         byte = 0
	architectureX8664       byte = 1
	architectureARM         byte = 2
	architectureARM64       byte = 3
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procIsWow64Process           = kernel32.NewProc("IsWow64Process")
	procIsWow64Process2          = kernel32.NewProc("IsWow64Process2")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procVirtualQueryEx           = kernel32.NewProc("VirtualQueryEx")
	procWriteProcessMemory       = kernel32.NewProc("WriteProcessMemory")
)

type systemBackend struct{}

type windowsProcess struct {
	handle       syscall.Handle
	architecture byte
}

type processEntry32 struct {
	Size              uint32
	Usage             uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	Threads           uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExecutableFile    [maximumPath]uint16
}

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	Padding           uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
	TrailingPadding   uint32
}

func NewSystemBackend() (Backend, error) {
	return &systemBackend{}, nil
}

func (backend *systemBackend) ABI() byte {
	return 0
}

func (backend *systemBackend) Processes() ([]Process, error) {
	handle, _, callErr := procCreateToolhelp32Snapshot.Call(th32csSnapshotProcess, 0)
	if handle == invalidHandleValue {
		return nil, windowsCallError("CreateToolhelp32Snapshot", callErr)
	}
	defer procCloseHandle.Call(handle)
	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	result, _, callErr := procProcess32FirstW.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if result == 0 {
		return nil, windowsCallError("Process32FirstW", callErr)
	}
	processes := make([]Process, 0, 128)
	for {
		processes = append(processes, Process{PID: int32(entry.ProcessID), Name: syscall.UTF16ToString(entry.ExecutableFile[:])})
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		result, _, _ = procProcess32NextW.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if result == 0 {
			break
		}
	}
	return processes, nil
}

func (backend *systemBackend) OpenProcess(pid int32) (ProcessMemory, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid process ID %d", pid)
	}
	access := uintptr(processQueryInformation | processVMOperation | processVMRead | processVMWrite)
	handle, _, callErr := procOpenProcess.Call(access, 0, uintptr(uint32(pid)))
	if handle == 0 {
		return nil, windowsCallError("OpenProcess", callErr)
	}
	process := &windowsProcess{handle: syscall.Handle(handle)}
	process.architecture = detectArchitecture(process.handle)
	return process, nil
}

func (process *windowsProcess) Close() error {
	if process.handle == 0 {
		return nil
	}
	handle := process.handle
	process.handle = 0
	result, _, callErr := procCloseHandle.Call(uintptr(handle))
	if result == 0 {
		return windowsCallError("CloseHandle", callErr)
	}
	return nil
}

func (process *windowsProcess) Architecture() byte {
	return process.architecture
}

func (process *windowsProcess) Regions(flags byte) ([]Region, error) {
	if process.handle == 0 {
		return nil, errors.New("process handle is closed")
	}
	regions := make([]Region, 0, 1024)
	for address := uintptr(0); ; {
		var information memoryBasicInformation
		result, _, _ := procVirtualQueryEx.Call(
			uintptr(process.handle),
			address,
			uintptr(unsafe.Pointer(&information)),
			unsafe.Sizeof(information),
		)
		if result == 0 {
			break
		}
		if information.RegionSize == 0 {
			break
		}
		if information.State == memCommit {
			regions = append(regions, Region{
				BaseAddress: uint64(information.BaseAddress),
				Size:        uint64(information.RegionSize),
				Protection:  convertProtection(information.Protect),
				Type:        convertMemoryType(information.Type),
			})
		}
		nextAddress := information.BaseAddress + information.RegionSize
		if nextAddress <= address {
			break
		}
		address = nextAddress
	}
	if len(regions) == 0 {
		return nil, errors.New("VirtualQueryEx returned no committed regions")
	}
	return regions, nil
}

func (process *windowsProcess) Read(address uint64, size uint32) ([]byte, error) {
	if process.handle == 0 {
		return nil, errors.New("process handle is closed")
	}
	if size == 0 {
		return []byte{}, nil
	}
	data := make([]byte, size)
	var bytesRead uintptr
	result, _, callErr := procReadProcessMemory.Call(
		uintptr(process.handle),
		uintptr(address),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	if bytesRead > uintptr(size) {
		return nil, fmt.Errorf("ReadProcessMemory returned invalid byte count %d", bytesRead)
	}
	if result == 0 && bytesRead == 0 {
		return nil, windowsCallError("ReadProcessMemory", callErr)
	}
	return data[:bytesRead], nil
}

func (process *windowsProcess) Write(address uint64, data []byte) (uint32, error) {
	if process.handle == 0 {
		return 0, errors.New("process handle is closed")
	}
	if len(data) == 0 {
		return 0, nil
	}
	var bytesWritten uintptr
	result, _, callErr := procWriteProcessMemory.Call(
		uintptr(process.handle),
		uintptr(address),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&bytesWritten)),
	)
	if result == 0 {
		return uint32(bytesWritten), windowsCallError("WriteProcessMemory", callErr)
	}
	if bytesWritten > uintptr(len(data)) {
		return 0, fmt.Errorf("WriteProcessMemory returned invalid byte count %d", bytesWritten)
	}
	return uint32(bytesWritten), nil
}

func detectArchitecture(handle syscall.Handle) byte {
	if procIsWow64Process2.Find() == nil {
		var processMachine uint16
		var nativeMachine uint16
		result, _, _ := procIsWow64Process2.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&processMachine)),
			uintptr(unsafe.Pointer(&nativeMachine)),
		)
		if result != 0 {
			machine := processMachine
			if machine == imageFileMachineUnknown {
				machine = nativeMachine
			}
			return machineArchitecture(machine)
		}
	}
	var wow64 int32
	result, _, _ := procIsWow64Process.Call(uintptr(handle), uintptr(unsafe.Pointer(&wow64)))
	if result != 0 && wow64 != 0 {
		return architectureX86
	}
	switch runtime.GOARCH {
	case "amd64":
		return architectureX8664
	case "arm64":
		return architectureARM64
	case "arm":
		return architectureARM
	default:
		return architectureX86
	}
}

func machineArchitecture(machine uint16) byte {
	switch machine {
	case imageFileMachineI386:
		return architectureX86
	case imageFileMachineAMD64:
		return architectureX8664
	case imageFileMachineARMNT:
		return architectureARM
	case imageFileMachineARM64:
		return architectureARM64
	default:
		return architectureX86
	}
}

func convertProtection(protection uint32) uint32 {
	switch protection & 0xff {
	case pageNoAccess:
		return pageNoAccess
	case pageReadOnly:
		return pageReadOnly
	case pageReadWrite:
		return pageReadWrite
	case pageWriteCopy:
		return pageWriteCopy
	case pageExecute:
		return pageExecute
	case pageExecuteRead:
		return pageExecuteRead
	case pageExecuteReadWrite:
		return pageExecuteReadWrite
	case pageExecuteWriteCopy:
		return pageWriteCopy
	default:
		return pageNoAccess
	}
}

func convertMemoryType(memoryType uint32) uint32 {
	switch memoryType {
	case memPrivate:
		return memPrivate
	case memMapped, memImage:
		return memMapped
	default:
		return memoryType
	}
}

func windowsCallError(operation string, callErr error) error {
	if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s failed: %w", operation, callErr)
}
