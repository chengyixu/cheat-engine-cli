//go:build darwin && cgo

package localbridge

/*
#include <libproc.h>
#include <mach/mach.h>
#include <mach/mach_error.h>
#include <mach/mach_vm.h>
#include <mach/machine.h>
#include <stdint.h>
#include <sys/proc_info.h>

typedef struct {
	uint64_t address;
	uint64_t size;
	int protection;
	unsigned char share_mode;
} ce_region_info;

static int ce_process_cpu_type(int pid, cpu_type_t *cpu_type) {
	struct proc_archinfo info;
	int result = proc_pidinfo(pid, PROC_PIDARCHINFO, 0, &info, sizeof(info));
	if (result != sizeof(info)) {
		return 0;
	}
	*cpu_type = info.p_cputype;
	return 1;
}

static kern_return_t ce_open_task(int pid, mach_port_t *task) {
	return task_for_pid(mach_task_self(), pid, task);
}

static kern_return_t ce_close_task(mach_port_t task) {
	return mach_port_deallocate(mach_task_self(), task);
}

static kern_return_t ce_next_region(
	mach_port_t task,
	mach_vm_address_t *address,
	natural_t *depth,
	ce_region_info *region
) {
	for (;;) {
		mach_vm_address_t current = *address;
		mach_vm_size_t size = 0;
		vm_region_submap_info_data_64_t info;
		mach_msg_type_number_t count = VM_REGION_SUBMAP_INFO_COUNT_64;
		kern_return_t result = mach_vm_region_recurse(
			task,
			&current,
			&size,
			depth,
			(vm_region_recurse_info_t)&info,
			&count
		);
		if (result != KERN_SUCCESS) {
			return result;
		}
		if (info.is_submap) {
			*depth += 1;
			*address = current;
			continue;
		}
		region->address = current;
		region->size = size;
		region->protection = info.protection;
		region->share_mode = info.share_mode;
		*address = current + size;
		return KERN_SUCCESS;
	}
}

static kern_return_t ce_read_memory(
	mach_port_t task,
	uint64_t address,
	void *buffer,
	uint32_t size,
	mach_vm_size_t *bytes_read
) {
	return mach_vm_read_overwrite(
		task,
		(mach_vm_address_t)address,
		(mach_vm_size_t)size,
		(mach_vm_address_t)buffer,
		bytes_read
	);
}

static kern_return_t ce_write_memory(
	mach_port_t task,
	uint64_t address,
	void *buffer,
	uint32_t size
) {
	return mach_vm_write(
		task,
		(mach_vm_address_t)address,
		(vm_offset_t)buffer,
		(mach_msg_type_number_t)size
	);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"unsafe"
)

const (
	abiUnix                 byte   = 1
	architectureX86         byte   = 0
	architectureX8664       byte   = 1
	architectureARM         byte   = 2
	architectureARM64       byte   = 3
	pageNoAccess            uint32 = 0x01
	pageReadOnly            uint32 = 0x02
	pageReadWrite           uint32 = 0x04
	pageExecute             uint32 = 0x10
	pageExecuteRead         uint32 = 0x20
	pageExecuteReadWrite    uint32 = 0x40
	memoryPrivate           uint32 = 0x20000
	memoryMapped            uint32 = 0x40000
	virtualQueryNoShared    byte   = 0x04
	maximumProcessNameBytes        = 4096
)

type systemBackend struct{}

type darwinProcess struct {
	task         C.mach_port_t
	architecture byte
}

func NewSystemBackend() (Backend, error) {
	return &systemBackend{}, nil
}

func (backend *systemBackend) ABI() byte {
	return abiUnix
}

func (backend *systemBackend) Processes() ([]Process, error) {
	bufferSize := int(C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0))
	if bufferSize <= 0 {
		return nil, errors.New("proc_listpids returned no process buffer size")
	}
	pids := make([]C.int, bufferSize/int(C.sizeof_int))
	written := int(C.proc_listpids(C.PROC_ALL_PIDS, 0, unsafe.Pointer(&pids[0]), C.int(bufferSize)))
	if written <= 0 {
		return nil, errors.New("proc_listpids failed")
	}
	count := written / int(C.sizeof_int)
	processes := make([]Process, 0, count)
	nameBuffer := make([]byte, maximumProcessNameBytes)
	for _, processID := range pids[:count] {
		if processID <= 0 {
			continue
		}
		nameLength := int(C.proc_name(processID, unsafe.Pointer(&nameBuffer[0]), C.uint32_t(len(nameBuffer))))
		if nameLength <= 0 {
			continue
		}
		processes = append(processes, Process{PID: int32(processID), Name: string(nameBuffer[:nameLength])})
	}
	sort.Slice(processes, func(first, second int) bool {
		return processes[first].PID < processes[second].PID
	})
	return processes, nil
}

func (backend *systemBackend) OpenProcess(pid int32) (ProcessMemory, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid process ID %d", pid)
	}
	var task C.mach_port_t
	result := C.ce_open_task(C.int(pid), &task)
	if result != C.KERN_SUCCESS {
		return nil, machError(fmt.Sprintf("task_for_pid(%d)", pid), result)
	}
	return &darwinProcess{task: task, architecture: darwinArchitecture(pid)}, nil
}

func (process *darwinProcess) Close() error {
	if process.task == 0 {
		return nil
	}
	task := process.task
	process.task = 0
	result := C.ce_close_task(task)
	if result != C.KERN_SUCCESS {
		return machError("mach_port_deallocate", result)
	}
	return nil
}

func (process *darwinProcess) Architecture() byte {
	return process.architecture
}

func (process *darwinProcess) Regions(flags byte) ([]Region, error) {
	if process.task == 0 {
		return nil, errors.New("process handle is closed")
	}
	regions := make([]Region, 0, 1024)
	var address C.mach_vm_address_t
	var depth C.natural_t
	for {
		previousAddress := address
		var information C.ce_region_info
		result := C.ce_next_region(process.task, &address, &depth, &information)
		if result == C.KERN_INVALID_ADDRESS {
			break
		}
		if result != C.KERN_SUCCESS {
			return nil, machError("mach_vm_region_recurse", result)
		}
		if information.size == 0 || address <= previousAddress {
			break
		}
		memoryType := convertDarwinMemoryType(byte(information.share_mode))
		if flags&virtualQueryNoShared != 0 && memoryType != memoryPrivate {
			continue
		}
		regions = append(regions, Region{
			BaseAddress: uint64(information.address),
			Size:        uint64(information.size),
			Protection:  convertDarwinProtection(int(information.protection)),
			Type:        memoryType,
		})
	}
	if len(regions) == 0 {
		return nil, errors.New("mach_vm_region_recurse returned no regions")
	}
	return regions, nil
}

func (process *darwinProcess) Read(address uint64, size uint32) ([]byte, error) {
	if process.task == 0 {
		return nil, errors.New("process handle is closed")
	}
	if size == 0 {
		return []byte{}, nil
	}
	data := make([]byte, size)
	var bytesRead C.mach_vm_size_t
	result := C.ce_read_memory(process.task, C.uint64_t(address), unsafe.Pointer(&data[0]), C.uint32_t(size), &bytesRead)
	if result != C.KERN_SUCCESS {
		return nil, machError("mach_vm_read_overwrite", result)
	}
	if uint64(bytesRead) > uint64(size) {
		return nil, fmt.Errorf("mach_vm_read_overwrite returned invalid byte count %d", uint64(bytesRead))
	}
	return data[:int(bytesRead)], nil
}

func (process *darwinProcess) Write(address uint64, data []byte) (uint32, error) {
	if process.task == 0 {
		return 0, errors.New("process handle is closed")
	}
	if len(data) == 0 {
		return 0, nil
	}
	if uint64(len(data)) > uint64(^uint32(0)) {
		return 0, fmt.Errorf("write size %d exceeds macOS Mach API limit", len(data))
	}
	result := C.ce_write_memory(process.task, C.uint64_t(address), unsafe.Pointer(&data[0]), C.uint32_t(len(data)))
	if result != C.KERN_SUCCESS {
		return 0, machError("mach_vm_write", result)
	}
	return uint32(len(data)), nil
}

func darwinArchitecture(pid int32) byte {
	var cpuType C.cpu_type_t
	if C.ce_process_cpu_type(C.int(pid), &cpuType) != 0 {
		switch cpuType {
		case C.CPU_TYPE_X86:
			return architectureX86
		case C.CPU_TYPE_X86_64:
			return architectureX8664
		case C.CPU_TYPE_ARM:
			return architectureARM
		case C.CPU_TYPE_ARM64:
			return architectureARM64
		}
	}
	if runtime.GOARCH == "arm64" {
		return architectureARM64
	}
	return architectureX8664
}

func convertDarwinProtection(protection int) uint32 {
	readable := protection&int(C.VM_PROT_READ) != 0
	writable := protection&int(C.VM_PROT_WRITE) != 0
	executable := protection&int(C.VM_PROT_EXECUTE) != 0
	switch {
	case readable && writable && executable:
		return pageExecuteReadWrite
	case readable && executable:
		return pageExecuteRead
	case executable:
		return pageExecute
	case readable && writable:
		return pageReadWrite
	case readable:
		return pageReadOnly
	case writable:
		return pageReadWrite
	default:
		return pageNoAccess
	}
}

func convertDarwinMemoryType(shareMode byte) uint32 {
	if shareMode == byte(C.SM_PRIVATE) || shareMode == byte(C.SM_PRIVATE_ALIASED) {
		return memoryPrivate
	}
	return memoryMapped
}

func machError(operation string, result C.kern_return_t) error {
	description := C.mach_error_string(result)
	if description == nil {
		return fmt.Errorf("%s failed with Mach error %d", operation, int(result))
	}
	return fmt.Errorf("%s: %s (%d)", operation, C.GoString(description), int(result))
}
