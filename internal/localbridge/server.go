package localbridge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
)

const (
	commandGetVersion               byte   = 0
	commandCloseConnection          byte   = 1
	commandOpenProcess              byte   = 3
	commandCreateToolhelp32Snapshot byte   = 4
	commandProcess32First           byte   = 5
	commandProcess32Next            byte   = 6
	commandCloseHandle              byte   = 7
	commandReadProcessMemory        byte   = 9
	commandWriteProcessMemory       byte   = 10
	commandGetArchitecture          byte   = 21
	commandVirtualQueryExFull       byte   = 31
	commandGetABI                   byte   = 33
	commandSetConnectionName        byte   = 34
	commandGetServerPath            byte   = 44
	commandIsAndroid                byte   = 45
	commandGetCurrentPath           byte   = 48
	maximumTransferSize                    = 16 << 20
	maximumConnectionName                  = 1 << 20
	toolhelpSnapshotProcess         uint32 = 0x2
)

type Process struct {
	PID  int32
	Name string
}

type Region struct {
	BaseAddress uint64
	Size        uint64
	Protection  uint32
	Type        uint32
}

type ProcessMemory interface {
	Close() error
	Architecture() byte
	Regions(flags byte) ([]Region, error)
	Read(address uint64, size uint32) ([]byte, error)
	Write(address uint64, data []byte) (uint32, error)
}

type Backend interface {
	ABI() byte
	Processes() ([]Process, error)
	OpenProcess(pid int32) (ProcessMemory, error)
}

type Server struct {
	Backend     Backend
	VersionName string
	waitGroup   sync.WaitGroup
}

type snapshot struct {
	processes []Process
	index     int
}

type connectionState struct {
	backend    Backend
	handles    map[uint32]any
	nextHandle uint32
}

func (server *Server) Serve(listener net.Listener) error {
	if server.Backend == nil {
		return errors.New("localbridge backend is required")
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				server.waitGroup.Wait()
				return nil
			}
			return err
		}
		server.waitGroup.Add(1)
		go func() {
			defer server.waitGroup.Done()
			server.ServeConnection(connection)
		}()
	}
}

func (server *Server) ServeConnection(connection net.Conn) {
	defer connection.Close()
	if server.Backend == nil {
		return
	}
	state := &connectionState{backend: server.Backend, handles: make(map[uint32]any), nextHandle: 1}
	defer state.closeHandles()
	versionName := server.VersionName
	if versionName == "" {
		versionName = "cecli-local"
	}
	for {
		command := []byte{0}
		if _, err := io.ReadFull(connection, command); err != nil {
			return
		}
		if !state.handleCommand(connection, command[0], versionName) {
			return
		}
	}
}

func (state *connectionState) handleCommand(connection net.Conn, command byte, versionName string) bool {
	switch command {
	case commandGetVersion:
		if len(versionName) > 255 {
			versionName = versionName[:255]
		}
		return writeValue(connection, int32(7)) && writeBytes(connection, []byte{byte(len(versionName))}) && writeBytes(connection, []byte(versionName))
	case commandCloseConnection:
		return false
	case commandOpenProcess:
		var pid int32
		if !readValue(connection, &pid) {
			return false
		}
		process, err := state.backend.OpenProcess(pid)
		if err != nil {
			return writeValue(connection, uint32(0))
		}
		return writeValue(connection, state.addHandle(process))
	case commandCreateToolhelp32Snapshot:
		var flags uint32
		var pid uint32
		if !readValue(connection, &flags) || !readValue(connection, &pid) {
			return false
		}
		if flags&toolhelpSnapshotProcess == 0 {
			return writeValue(connection, uint32(0))
		}
		processes, err := state.backend.Processes()
		if err != nil {
			return writeValue(connection, uint32(0))
		}
		return writeValue(connection, state.addHandle(&snapshot{processes: processes, index: -1}))
	case commandProcess32First, commandProcess32Next:
		var handle uint32
		if !readValue(connection, &handle) {
			return false
		}
		entry, ok := state.handles[handle].(*snapshot)
		if !ok {
			return writeProcess(connection, Process{}, false)
		}
		if command == commandProcess32First {
			entry.index = 0
		} else {
			entry.index++
		}
		if entry.index < 0 || entry.index >= len(entry.processes) {
			return writeProcess(connection, Process{}, false)
		}
		return writeProcess(connection, entry.processes[entry.index], true)
	case commandCloseHandle:
		var handle uint32
		if !readValue(connection, &handle) {
			return false
		}
		return writeValue(connection, int32(state.closeHandle(handle)))
	case commandReadProcessMemory:
		var handle uint32
		var address uint64
		var size uint32
		var compress byte
		if !readValue(connection, &handle) || !readValue(connection, &address) || !readValue(connection, &size) || !readValue(connection, &compress) {
			return false
		}
		process, ok := state.handles[handle].(ProcessMemory)
		if !ok || size > maximumTransferSize || compress != 0 {
			return writeValue(connection, int32(0))
		}
		data, err := process.Read(address, size)
		if err != nil || len(data) > int(size) {
			return writeValue(connection, int32(0))
		}
		return writeValue(connection, int32(len(data))) && writeBytes(connection, data)
	case commandWriteProcessMemory:
		var handle int32
		var address int64
		var size int32
		if !readValue(connection, &handle) || !readValue(connection, &address) || !readValue(connection, &size) {
			return false
		}
		if size < 0 || size > maximumTransferSize {
			return false
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(connection, data); err != nil {
			return false
		}
		process, ok := state.handles[uint32(handle)].(ProcessMemory)
		if !ok {
			return writeValue(connection, int32(0))
		}
		written, err := process.Write(uint64(address), data)
		if err != nil {
			return writeValue(connection, int32(0))
		}
		return writeValue(connection, int32(written))
	case commandGetArchitecture:
		var handle uint32
		if !readValue(connection, &handle) {
			return false
		}
		process, ok := state.handles[handle].(ProcessMemory)
		if !ok {
			return writeBytes(connection, []byte{0})
		}
		return writeBytes(connection, []byte{process.Architecture()})
	case commandVirtualQueryExFull:
		var handle uint32
		var flags byte
		if !readValue(connection, &handle) || !readValue(connection, &flags) {
			return false
		}
		process, ok := state.handles[handle].(ProcessMemory)
		if !ok {
			return writeValue(connection, uint32(0))
		}
		regions, err := process.Regions(flags)
		if err != nil {
			return writeValue(connection, uint32(0))
		}
		if !writeValue(connection, uint32(len(regions))) {
			return false
		}
		for _, region := range regions {
			if !writeValue(connection, region.BaseAddress) || !writeValue(connection, region.Size) || !writeValue(connection, region.Protection) || !writeValue(connection, region.Type) {
				return false
			}
		}
		return true
	case commandGetABI:
		return writeBytes(connection, []byte{state.backend.ABI()})
	case commandSetConnectionName:
		var size uint32
		if !readValue(connection, &size) || size > maximumConnectionName {
			return false
		}
		_, err := io.CopyN(io.Discard, connection, int64(size))
		return err == nil
	case commandGetServerPath:
		executable, err := os.Executable()
		if err != nil {
			executable = "cebridge.exe"
		}
		return writeString16(connection, filepath.Dir(executable))
	case commandIsAndroid:
		return writeBytes(connection, []byte{0})
	case commandGetCurrentPath:
		currentPath, err := os.Getwd()
		if err != nil {
			currentPath = "."
		}
		return writeString16(connection, currentPath)
	default:
		return false
	}
}

func (state *connectionState) addHandle(value any) uint32 {
	handle := state.nextHandle
	state.nextHandle++
	if state.nextHandle == 0 {
		state.nextHandle = 1
	}
	state.handles[handle] = value
	return handle
}

func (state *connectionState) closeHandle(handle uint32) int {
	value, ok := state.handles[handle]
	if !ok {
		return 0
	}
	delete(state.handles, handle)
	if process, ok := value.(ProcessMemory); ok {
		_ = process.Close()
	}
	return 1
}

func (state *connectionState) closeHandles() {
	for handle := range state.handles {
		state.closeHandle(handle)
	}
}

func writeProcess(writer io.Writer, process Process, result bool) bool {
	resultValue := int32(0)
	if result {
		resultValue = 1
	}
	return writeValue(writer, resultValue) && writeValue(writer, process.PID) && writeValue(writer, int32(len(process.Name))) && writeBytes(writer, []byte(process.Name))
}

func writeString16(writer io.Writer, value string) bool {
	if len(value) > int(^uint16(0)) {
		return false
	}
	return writeValue(writer, uint16(len(value))) && writeBytes(writer, []byte(value))
}

func readValue(reader io.Reader, value any) bool {
	return binary.Read(reader, binary.LittleEndian, value) == nil
}

func writeValue(writer io.Writer, value any) bool {
	return binary.Write(writer, binary.LittleEndian, value) == nil
}

func writeBytes(writer io.Writer, value []byte) bool {
	for len(value) > 0 {
		written, err := writer.Write(value)
		if err != nil || written == 0 {
			return false
		}
		value = value[written:]
	}
	return true
}

func ListenAndServe(address string, backend Backend, versionName string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	defer listener.Close()
	return (&Server{Backend: backend, VersionName: versionName}).Serve(listener)
}
