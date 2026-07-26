package localbridge_test

import (
	"context"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/chengyixu/cheat-engine-cli/internal/ceserver"
	"github.com/chengyixu/cheat-engine-cli/internal/localbridge"
)

type fakeBackend struct {
	process *fakeProcess
}

type fakeProcess struct {
	mutex  sync.Mutex
	memory []byte
}

func (backend *fakeBackend) ABI() byte {
	return 0
}

func (backend *fakeBackend) Processes() ([]localbridge.Process, error) {
	return []localbridge.Process{{PID: 4242, Name: "KairoGames.exe"}}, nil
}

func (backend *fakeBackend) OpenProcess(pid int32) (localbridge.ProcessMemory, error) {
	return backend.process, nil
}

func (process *fakeProcess) Close() error {
	return nil
}

func (process *fakeProcess) Architecture() byte {
	return byte(ceserver.ArchitectureX86)
}

func (process *fakeProcess) Regions(flags byte) ([]localbridge.Region, error) {
	return []localbridge.Region{{BaseAddress: 0x1000, Size: uint64(len(process.memory)), Protection: uint32(ceserver.ProtectionReadWrite), Type: uint32(ceserver.MemoryTypePrivate)}}, nil
}

func (process *fakeProcess) Read(address uint64, size uint32) ([]byte, error) {
	process.mutex.Lock()
	defer process.mutex.Unlock()
	offset := int(address - 0x1000)
	return append([]byte(nil), process.memory[offset:offset+int(size)]...), nil
}

func (process *fakeProcess) Write(address uint64, data []byte) (uint32, error) {
	process.mutex.Lock()
	defer process.mutex.Unlock()
	offset := int(address - 0x1000)
	copy(process.memory[offset:], data)
	return uint32(len(data)), nil
}

func TestServerSupportsDiscoveryScanAndVerifiedWrite(t *testing.T) {
	memory := make([]byte, 64)
	copy(memory[16:], []byte{0x9E, 0x52, 0x00, 0x00})
	backend := &fakeBackend{process: &fakeProcess{memory: memory}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &localbridge.Server{Backend: backend, VersionName: "cebridge-test"}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() {
		_ = listener.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}()

	client, err := ceserver.Dial(context.Background(), listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	info, err := client.ServerInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.VersionName != "cebridge-test" || info.ABI != "windows" {
		t.Fatalf("server info = %#v", info)
	}
	processes, err := client.ListProcesses()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(processes, []ceserver.Process{{PID: 4242, Name: "KairoGames.exe"}}) {
		t.Fatalf("processes = %#v", processes)
	}
	handle, err := client.OpenProcess(4242)
	if err != nil {
		t.Fatal(err)
	}
	architecture, err := client.Architecture(handle)
	_ = client.CloseHandle(handle)
	if err != nil || architecture != ceserver.ArchitectureX86 {
		t.Fatalf("architecture = %v, %v", architecture, err)
	}
	matches, err := client.ScanMemory(context.Background(), 4242, []byte{0x9E, 0x52, 0x00, 0x00}, []byte("xxxx"), 0x1000, 0x1040, 1, ceserver.ProtectionReadable, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(matches, []uint64{0x1010}) {
		t.Fatalf("matches = %#v", matches)
	}
	written, err := client.WriteMemory(4242, 0x1010, []byte{0x0F, 0x27, 0x00, 0x00})
	if err != nil || written != 4 {
		t.Fatalf("write = %d, %v", written, err)
	}
	readBack, err := client.ReadMemory(4242, 0x1010, 4)
	if err != nil || !reflect.DeepEqual(readBack, []byte{0x0F, 0x27, 0x00, 0x00}) {
		t.Fatalf("read back = % X, %v", readBack, err)
	}
}
