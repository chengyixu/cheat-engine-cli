//go:build darwin && cgo

package localbridge

import (
	"bytes"
	"os"
	"runtime"
	"testing"
	"unsafe"
)

func TestDarwinBackendReadsAndWritesCurrentProcess(t *testing.T) {
	backend, err := NewSystemBackend()
	if err != nil {
		t.Fatal(err)
	}
	process, err := backend.OpenProcess(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()

	value := []byte{0x10, 0x20, 0x30, 0x40}
	address := uint64(uintptr(unsafe.Pointer(&value[0])))
	readBack, err := process.Read(address, uint32(len(value)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readBack, value) {
		t.Fatalf("read back = % X", readBack)
	}

	replacement := []byte{0x50, 0x60, 0x70, 0x80}
	written, err := process.Write(address, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if written != uint32(len(replacement)) || !bytes.Equal(value, replacement) {
		t.Fatalf("written = %d, value = % X", written, value)
	}
	runtime.KeepAlive(value)
}

func TestDarwinBackendListsCurrentProcessAndRegions(t *testing.T) {
	backend, err := NewSystemBackend()
	if err != nil {
		t.Fatal(err)
	}
	processes, err := backend.Processes()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, process := range processes {
		if process.PID == int32(os.Getpid()) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("current PID %d was not listed", os.Getpid())
	}
	process, err := backend.OpenProcess(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	regions, err := process.Regions(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) == 0 {
		t.Fatal("current process has no regions")
	}
}
