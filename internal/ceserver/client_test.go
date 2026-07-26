package ceserver

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestClientProtocolSurface(t *testing.T) {
	endpoint, stop := startFakeServer(t)
	defer stop()
	client, err := Dial(context.Background(), endpoint, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	serverInfo, err := client.ServerInfo()
	if err != nil {
		t.Fatal(err)
	}
	if serverInfo.ProtocolVersion != 7 || serverInfo.VersionName != "fake-ceserver" || serverInfo.ABI != "unix" {
		t.Fatalf("serverInfo = %#v", serverInfo)
	}
	if err := client.SetConnectionName("cecli-test"); err != nil {
		t.Fatal(err)
	}

	processes, err := client.ListProcesses()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(processes, []Process{{PID: 4242, Name: "sample-game"}}) {
		t.Fatalf("processes = %#v", processes)
	}

	processInfo, err := client.ProcessInfo(4242)
	if err != nil {
		t.Fatal(err)
	}
	if processInfo.Architecture != "arm64" || processInfo.ModuleCount != 1 || processInfo.ThreadCount != 2 {
		t.Fatalf("processInfo = %#v", processInfo)
	}

	regions, err := client.Regions(4242, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) != 1 || regions[0].BaseAddress != 0x1000 || regions[0].Size != 8 {
		t.Fatalf("regions = %#v", regions)
	}

	data, err := client.ReadMemory(4242, 0x1000, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, []byte{0x48, 0x8B, 0x01, 0xFF}) {
		t.Fatalf("data = %#v", data)
	}

	written, err := client.WriteMemory(4242, 0x1000, []byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if written != 4 {
		t.Fatalf("written = %d", written)
	}

	matches, err := client.ScanMemory(context.Background(), 4242, []byte{0x48, 0x8B, 0, 0xFF}, []byte("xx?x"), 0x1000, 0x1008, 4, ProtectionReadable, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(matches, []uint64{0x1000, 0x1004}) {
		t.Fatalf("matches = %#v", matches)
	}
	serverMatches, err := client.ServerAOBScan(4242, []byte{0x48, 0x8B, 0, 0xFF}, []byte("xx?x"), 0x1000, 0x1008, 4, ProtectionReadable)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(serverMatches, []uint64{0x1000, 0x1004}) {
		t.Fatalf("serverMatches = %#v", serverMatches)
	}

	pathInfo, err := client.PathInfo()
	if err != nil {
		t.Fatal(err)
	}
	if pathInfo.ExecutablePath != "/opt/ceserver/" || pathInfo.CurrentPath != "/tmp" || pathInfo.Android {
		t.Fatalf("pathInfo = %#v", pathInfo)
	}
	changed, err := client.SetCurrentPath("/var/tmp")
	if err != nil || !changed {
		t.Fatalf("SetCurrentPath = %t, %v", changed, err)
	}

	options, err := client.Options()
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 1 || options[0].Name != "optMSO" || options[0].CurrentValue != "2" {
		t.Fatalf("options = %#v", options)
	}
	if err := client.SetOption("optMSO", "3"); err != nil {
		t.Fatal(err)
	}
	optionValue, err := client.Option("optMSO")
	if err != nil || optionValue != "3" {
		t.Fatalf("Option = %q, %v", optionValue, err)
	}

	files, err := client.ListRemoteFiles("/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []RemoteFile{{Name: "sample.bin", TypeCode: 8, Type: "file"}}) {
		t.Fatalf("files = %#v", files)
	}
	permissions, err := client.RemoteFilePermissions("/tmp/sample.bin")
	if err != nil || permissions != 0o755 {
		t.Fatalf("permissions = %#o, %v", permissions, err)
	}
	if changed, err := client.SetRemoteFilePermissions("/tmp/sample.bin", 0o700); err != nil || !changed {
		t.Fatalf("SetRemoteFilePermissions = %t, %v", changed, err)
	}
	content, err := client.GetRemoteFile("/tmp/sample.bin")
	if err != nil || string(content) != "abc" {
		t.Fatalf("GetRemoteFile = %q, %v", content, err)
	}
	if changed, err := client.PutRemoteFile("/tmp/output.bin", []byte("xyz")); err != nil || !changed {
		t.Fatalf("PutRemoteFile = %t, %v", changed, err)
	}
	if changed, err := client.CreateRemoteDirectory("/tmp/new"); err != nil || !changed {
		t.Fatalf("CreateRemoteDirectory = %t, %v", changed, err)
	}
	if changed, err := client.DeleteRemotePath("/tmp/new"); err != nil || !changed {
		t.Fatalf("DeleteRemotePath = %t, %v", changed, err)
	}

	allocated, err := client.AllocateMemory(4242, 0, 4096, ProtectionReadWrite)
	if err != nil || allocated != 0x5000 {
		t.Fatalf("AllocateMemory = %#x, %v", allocated, err)
	}
	if freed, err := client.FreeMemory(4242, allocated, 4096); err != nil || !freed {
		t.Fatalf("FreeMemory = %t, %v", freed, err)
	}
	protectionChange, err := client.ChangeMemoryProtection(4242, 0x1000, 4096, ProtectionExecuteRead)
	if err != nil || protectionChange.OldProtection != ProtectionReadWrite {
		t.Fatalf("ChangeMemoryProtection = %#v, %v", protectionChange, err)
	}
	if changed, err := client.SetSpeed(4242, 1.5); err != nil || !changed {
		t.Fatalf("SetSpeed = %t, %v", changed, err)
	}
	moduleAddress, err := client.LoadModule(4242, "/tmp/plugin.so")
	if err != nil || moduleAddress != 0x7000 {
		t.Fatalf("LoadModule = %#x, %v", moduleAddress, err)
	}
	moduleAddress, err = client.LoadModuleEx(4242, 0xDEADBEEF, "/tmp/plugin-ex.so")
	if err != nil || moduleAddress != 0x7100 {
		t.Fatalf("LoadModuleEx = %#x, %v", moduleAddress, err)
	}
	if loaded, err := client.LoadExtension(4242); err != nil || !loaded {
		t.Fatalf("LoadExtension = %t, %v", loaded, err)
	}
	threadHandle, err := client.CreateRemoteThread(4242, 0x1234, 0x5678)
	if err != nil || threadHandle != 73 {
		t.Fatalf("CreateRemoteThread = %d, %v", threadHandle, err)
	}
	if suspendCount, err := client.SuspendThread(4242, 9001); err != nil || suspendCount != 1 {
		t.Fatalf("SuspendThread = %d, %v", suspendCount, err)
	}
	if suspendCount, err := client.ResumeThread(4242, 9001); err != nil || suspendCount != 0 {
		t.Fatalf("ResumeThread = %d, %v", suspendCount, err)
	}
	if changed, err := client.SetBreakpoint(4242, -1, 0, 0x1234, BreakpointExecute, 1); err != nil || !changed {
		t.Fatalf("SetBreakpoint = %t, %v", changed, err)
	}
	if changed, err := client.RemoveBreakpoint(4242, -1, 0, false); err != nil || !changed {
		t.Fatalf("RemoveBreakpoint = %t, %v", changed, err)
	}
	threadContext, err := client.GetThreadContext(4242, 9001)
	if err != nil || threadContext.StructSize != 16 || threadContext.TypeCode != 3 || len(threadContext.Bytes) != 16 {
		t.Fatalf("GetThreadContext = %#v, %v", threadContext, err)
	}
	if changed, err := client.SetThreadContext(4242, 9001, threadContext.Bytes); err != nil || !changed {
		t.Fatalf("SetThreadContext = %t, %v", changed, err)
	}
	trace, err := client.TraceDebugEvents(context.Background(), 4242, 3, 25*time.Millisecond, DebugContinueIgnoreSignal)
	if err != nil {
		t.Fatal(err)
	}
	if trace.EventCount != 2 || !trace.TimedOut || trace.Events[0].Kind != "create-process" || trace.Events[1].Address != 0x1234 {
		t.Fatalf("TraceDebugEvents = %#v", trace)
	}
	pipeHandle, err := client.OpenPipe("sample-pipe", 1000)
	if err != nil || pipeHandle != 81 {
		t.Fatalf("OpenPipe = %d, %v", pipeHandle, err)
	}
	pipeData, err := client.ReadPipe(pipeHandle, 3, 1000)
	if err != nil || string(pipeData) != "xyz" {
		t.Fatalf("ReadPipe = %q, %v", pipeData, err)
	}
	pipeWritten, err := client.WritePipe(pipeHandle, []byte("abc"), 1000)
	if err != nil || pipeWritten != 3 {
		t.Fatalf("WritePipe = %d, %v", pipeWritten, err)
	}
	if err := client.CloseHandle(pipeHandle); err != nil {
		t.Fatal(err)
	}
	if err := client.CloseHandle(threadHandle); err != nil {
		t.Fatal(err)
	}
	regionDetail, err := client.RegionInfo(4242, 0x1001)
	if err != nil || regionDetail.MapsLine != "1000-1008 rw-p sample" || regionDetail.BaseAddress != 0x1000 {
		t.Fatalf("RegionInfo = %#v, %v", regionDetail, err)
	}
	symbolList, err := client.Symbols("/tmp/sample-game", 0)
	if err != nil || symbolList.SymbolCount != 1 || symbolList.Symbols[0].Name != "main" {
		t.Fatalf("Symbols = %#v, %v", symbolList, err)
	}
	if err := client.TerminateServer(); err != nil {
		t.Fatal(err)
	}
}

func startFakeServer(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		handleFakeConnection(connection)
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("fake server did not stop")
		}
	}
}

func handleFakeConnection(connection net.Conn) {
	memoryBytes := []byte{0x48, 0x8B, 0x01, 0xFF, 0x48, 0x8B, 0x02, 0xFF}
	processIteration := 0
	debugEventIteration := 0
	for {
		command := []byte{0}
		if _, err := io.ReadFull(connection, command); err != nil {
			return
		}
		switch command[0] {
		case commandGetVersion:
			writeInt32(connection, 7)
			name := []byte("fake-ceserver")
			_, _ = connection.Write([]byte{byte(len(name))})
			_, _ = connection.Write(name)
		case commandGetABI:
			_, _ = connection.Write([]byte{1})
		case commandSetConnectionName:
			length := binary.LittleEndian.Uint32(readBytes(connection, 4))
			readBytes(connection, int(length))
		case commandCreateToolhelp32Snapshot:
			readBytes(connection, 8)
			processIteration = 0
			writeUint32(connection, 41)
		case commandProcess32First, commandProcess32Next:
			readBytes(connection, 4)
			if processIteration == 0 {
				name := []byte("sample-game")
				writeInt32(connection, 1)
				writeInt32(connection, 4242)
				writeInt32(connection, int32(len(name)))
				_, _ = connection.Write(name)
				processIteration++
			} else {
				writeInt32(connection, 0)
				writeInt32(connection, 0)
				writeInt32(connection, 0)
			}
		case commandOpenProcess:
			readBytes(connection, 4)
			writeUint32(connection, 7)
		case commandCloseHandle:
			readBytes(connection, 4)
			writeInt32(connection, 1)
		case commandGetArchitecture:
			readBytes(connection, 4)
			_, _ = connection.Write([]byte{byte(ArchitectureARM64)})
		case commandCreateToolhelp32SnapshotEx:
			request := readBytes(connection, 8)
			flags := binary.LittleEndian.Uint32(request[:4])
			if flags == toolhelpSnapshotModule {
				name := []byte("/tmp/sample-game")
				header := make([]byte, 28)
				binary.LittleEndian.PutUint32(header[0:4], 1)
				binary.LittleEndian.PutUint64(header[4:12], 0x1000)
				binary.LittleEndian.PutUint32(header[16:20], 0x2000)
				binary.LittleEndian.PutUint32(header[24:28], uint32(len(name)))
				_, _ = connection.Write(header)
				_, _ = connection.Write(name)
				_, _ = connection.Write(make([]byte, 28))
			} else {
				writeInt32(connection, 2)
				writeInt32(connection, 101)
				writeInt32(connection, 102)
			}
		case commandVirtualQueryExFull:
			readBytes(connection, 5)
			writeUint32(connection, 1)
			region := make([]byte, 24)
			binary.LittleEndian.PutUint64(region[0:8], 0x1000)
			binary.LittleEndian.PutUint64(region[8:16], 8)
			binary.LittleEndian.PutUint32(region[16:20], uint32(ProtectionReadWrite))
			binary.LittleEndian.PutUint32(region[20:24], uint32(MemoryTypePrivate))
			_, _ = connection.Write(region)
		case commandReadProcessMemory:
			request := readBytes(connection, 17)
			address := binary.LittleEndian.Uint64(request[4:12])
			size := binary.LittleEndian.Uint32(request[12:16])
			offset := int(address - 0x1000)
			if offset < 0 || offset >= len(memoryBytes) {
				writeInt32(connection, 0)
				continue
			}
			end := min(offset+int(size), len(memoryBytes))
			response := memoryBytes[offset:end]
			writeInt32(connection, int32(len(response)))
			_, _ = connection.Write(response)
		case commandWriteProcessMemory:
			request := readBytes(connection, 16)
			size := int(int32(binary.LittleEndian.Uint32(request[12:16])))
			readBytes(connection, size)
			writeInt32(connection, int32(size))
		case commandAOBScan:
			request := readBytes(connection, 32)
			patternSize := int(int32(binary.LittleEndian.Uint32(request[28:32])))
			readBytes(connection, patternSize*2)
			writeInt32(connection, 2)
			_ = binary.Write(connection, binary.LittleEndian, uint64(0x1000))
			_ = binary.Write(connection, binary.LittleEndian, uint64(0x1004))
		case commandGetServerPath:
			writeString16ForTest(connection, "/opt/ceserver/")
		case commandGetCurrentPath:
			writeString16ForTest(connection, "/tmp")
		case commandIsAndroid:
			_, _ = connection.Write([]byte{0})
		case commandSetCurrentPath:
			readString16ForTest(connection)
			_, _ = connection.Write([]byte{1})
		case commandGetOptions:
			_ = binary.Write(connection, binary.LittleEndian, uint16(1))
			writeString16ForTest(connection, "optMSO")
			writeString16ForTest(connection, "")
			writeString16ForTest(connection, "Memory search option")
			writeString16ForTest(connection, "0=file;1=ptrace;2=process_vm_readv")
			writeString16ForTest(connection, "2")
			writeInt32(connection, 2)
		case commandSetOptionValue:
			readString16ForTest(connection)
			readString16ForTest(connection)
		case commandGetOptionValue:
			readString16ForTest(connection)
			writeString16ForTest(connection, "3")
		case commandEnumerateFiles:
			readString16ForTest(connection)
			writeString16ForTest(connection, "sample.bin")
			_, _ = connection.Write([]byte{8})
			writeString16ForTest(connection, "")
		case commandGetFilePermissions:
			readString16ForTest(connection)
			_, _ = connection.Write([]byte{1})
			writeUint32(connection, 0o755)
		case commandSetFilePermissions:
			readString16ForTest(connection)
			readBytes(connection, 4)
			_, _ = connection.Write([]byte{1})
		case commandGetFile:
			readString16ForTest(connection)
			writeUint32(connection, 3)
			_, _ = connection.Write([]byte("abc"))
		case commandPutFile:
			readString16ForTest(connection)
			sizeBytes := readBytes(connection, 4)
			readBytes(connection, int(binary.LittleEndian.Uint32(sizeBytes)))
			_, _ = connection.Write([]byte{1})
		case commandCreateDirectory, commandDeleteFile:
			readString16ForTest(connection)
			_, _ = connection.Write([]byte{1})
		case commandAllocateMemory:
			readBytes(connection, 20)
			_ = binary.Write(connection, binary.LittleEndian, uint64(0x5000))
		case commandFreeMemory:
			readBytes(connection, 16)
			writeUint32(connection, 1)
		case commandChangeMemoryProtection:
			readBytes(connection, 20)
			writeUint32(connection, 0)
			writeUint32(connection, uint32(ProtectionReadWrite))
		case commandSetSpeed:
			readBytes(connection, 8)
			writeUint32(connection, 1)
		case commandLoadModule:
			request := readBytes(connection, 8)
			pathLength := binary.LittleEndian.Uint32(request[4:8])
			readBytes(connection, int(pathLength))
			_ = binary.Write(connection, binary.LittleEndian, uint64(0x7000))
		case commandLoadModuleEx:
			request := readBytes(connection, 16)
			pathLength := binary.LittleEndian.Uint32(request[12:16])
			readBytes(connection, int(pathLength))
			_ = binary.Write(connection, binary.LittleEndian, uint64(0x7100))
		case commandLoadExtension:
			readBytes(connection, 4)
			writeInt32(connection, 1)
		case commandCreateRemoteThread:
			readBytes(connection, 20)
			writeUint32(connection, 73)
		case commandSuspendThread:
			readBytes(connection, 8)
			writeInt32(connection, 1)
		case commandResumeThread:
			readBytes(connection, 8)
			writeInt32(connection, 0)
		case commandSetBreakpoint:
			readBytes(connection, 28)
			writeInt32(connection, 1)
		case commandRemoveBreakpoint:
			readBytes(connection, 16)
			writeInt32(connection, 1)
		case commandGetThreadContext:
			readBytes(connection, 8)
			contextData := make([]byte, 16)
			binary.LittleEndian.PutUint32(contextData[0:4], uint32(len(contextData)))
			binary.LittleEndian.PutUint32(contextData[4:8], 3)
			binary.LittleEndian.PutUint64(contextData[8:16], 0x12345678)
			writeUint32(connection, 1)
			writeUint32(connection, uint32(len(contextData)))
			_, _ = connection.Write(contextData)
		case commandSetThreadContext:
			request := readBytes(connection, 12)
			contextSize := binary.LittleEndian.Uint32(request[8:12])
			readBytes(connection, int(contextSize))
			writeUint32(connection, 1)
		case commandStartDebug:
			readBytes(connection, 4)
			debugEventIteration = 0
			writeInt32(connection, 1)
		case commandWaitForDebugEvent:
			readBytes(connection, 8)
			switch debugEventIteration {
			case 0:
				writeInt32(connection, 1)
				event := make([]byte, 20)
				binary.LittleEndian.PutUint32(event[0:4], ^uint32(1))
				binary.LittleEndian.PutUint64(event[4:12], 9001)
				event[12], event[13], event[14] = 4, 2, 4
				_, _ = connection.Write(event)
			case 1:
				writeInt32(connection, 1)
				event := make([]byte, 20)
				binary.LittleEndian.PutUint32(event[0:4], 5)
				binary.LittleEndian.PutUint64(event[4:12], 9001)
				binary.LittleEndian.PutUint64(event[12:20], 0x1234)
				_, _ = connection.Write(event)
			default:
				writeInt32(connection, 0)
			}
			debugEventIteration++
		case commandContinueFromDebugEvent:
			readBytes(connection, 12)
			writeInt32(connection, 1)
		case commandOpenNamedPipe:
			readString16ForTest(connection)
			readBytes(connection, 4)
			writeUint32(connection, 81)
		case commandPipeRead:
			readBytes(connection, 12)
			writeInt32(connection, 3)
			_, _ = connection.Write([]byte("xyz"))
		case commandPipeWrite:
			request := readBytes(connection, 12)
			size := binary.LittleEndian.Uint32(request[4:8])
			readBytes(connection, int(size))
			writeInt32(connection, int32(size))
		case commandGetRegionInfo:
			readBytes(connection, 12)
			header := make([]byte, 25)
			header[0] = 1
			binary.LittleEndian.PutUint32(header[1:5], uint32(ProtectionReadWrite))
			binary.LittleEndian.PutUint32(header[5:9], uint32(MemoryTypePrivate))
			binary.LittleEndian.PutUint64(header[9:17], 0x1000)
			binary.LittleEndian.PutUint64(header[17:25], 8)
			_, _ = connection.Write(header)
			mapsLine := []byte("1000-1008 rw-p sample")
			_, _ = connection.Write([]byte{byte(len(mapsLine))})
			_, _ = connection.Write(mapsLine)
		case commandGetSymbols:
			request := readBytes(connection, 8)
			pathLength := binary.LittleEndian.Uint32(request[4:8])
			readBytes(connection, int(pathLength))
			symbolData := make([]byte, 21)
			binary.LittleEndian.PutUint64(symbolData[0:8], 0x1234)
			binary.LittleEndian.PutUint32(symbolData[8:12], 16)
			binary.LittleEndian.PutUint32(symbolData[12:16], 2)
			symbolData[16] = 4
			copy(symbolData[17:], "main")
			var compressed bytes.Buffer
			compressor := zlib.NewWriter(&compressed)
			_, _ = compressor.Write(symbolData)
			_ = compressor.Close()
			writeUint32(connection, 1)
			writeUint32(connection, uint32(compressed.Len()+12))
			writeUint32(connection, uint32(len(symbolData)))
			_, _ = connection.Write(compressed.Bytes())
		case commandCloseConnection:
			return
		case commandTerminateServer:
			return
		default:
			return
		}
	}
}

func readBytes(reader io.Reader, size int) []byte {
	data := make([]byte, size)
	_, _ = io.ReadFull(reader, data)
	return data
}

func writeInt32(writer io.Writer, value int32) {
	_ = binary.Write(writer, binary.LittleEndian, value)
}

func writeUint32(writer io.Writer, value uint32) {
	_ = binary.Write(writer, binary.LittleEndian, value)
}

func writeString16ForTest(writer io.Writer, value string) {
	_ = binary.Write(writer, binary.LittleEndian, uint16(len(value)))
	_, _ = io.WriteString(writer, value)
}

func readString16ForTest(reader io.Reader) string {
	var length uint16
	_ = binary.Read(reader, binary.LittleEndian, &length)
	return string(readBytes(reader, int(length)))
}
