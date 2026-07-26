package ceserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	connection  net.Conn
	endpoint    string
	timeout     time.Duration
	debugActive bool
}

func Dial(ctx context.Context, endpoint string, timeout time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, fmt.Errorf("connect to ceserver at %s: %w", endpoint, err)
	}
	if tcpConnection, ok := connection.(*net.TCPConn); ok {
		_ = tcpConnection.SetNoDelay(true)
	}
	return &Client{connection: connection, endpoint: endpoint, timeout: timeout}, nil
}

func (client *Client) Close() error {
	if client == nil || client.connection == nil {
		return nil
	}
	connection := client.connection
	client.connection = nil
	if client.debugActive {
		return connection.Close()
	}
	_ = connection.SetDeadline(time.Now().Add(client.timeout))
	_, _ = connection.Write([]byte{commandCloseConnection})
	return connection.Close()
}

func (client *Client) ServerInfo() (ServerInfo, error) {
	protocolVersion, versionName, err := client.Version()
	if err != nil {
		return ServerInfo{}, err
	}
	abi, err := client.ABI()
	if err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{
		Endpoint:        client.endpoint,
		ProtocolVersion: protocolVersion,
		VersionName:     versionName,
		ABI:             abi.String(),
	}, nil
}

func (client *Client) Version() (int32, string, error) {
	if err := client.writePacket([]byte{commandGetVersion}); err != nil {
		return 0, "", err
	}
	header := make([]byte, 5)
	if err := client.readFull(header); err != nil {
		return 0, "", client.protocolError("get version", err)
	}
	protocolVersion := int32(binary.LittleEndian.Uint32(header[:4]))
	nameLength := int(header[4])
	name := make([]byte, nameLength)
	if err := client.readFull(name); err != nil {
		return 0, "", client.protocolError("get version name", err)
	}
	return protocolVersion, string(name), nil
}

func (client *Client) ABI() (ABI, error) {
	if err := client.writePacket([]byte{commandGetABI}); err != nil {
		return 0, err
	}
	response := []byte{0}
	if err := client.readFull(response); err != nil {
		return 0, client.protocolError("get ABI", err)
	}
	return ABI(response[0]), nil
}

func (client *Client) OpenProcess(pid int32) (uint32, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 5))
	packet.WriteByte(commandOpenProcess)
	_ = binary.Write(packet, binary.LittleEndian, pid)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var handle uint32
	if err := client.readValue(&handle); err != nil {
		return 0, client.protocolError("open process", err)
	}
	if handle == 0 {
		return 0, &ProtocolError{Operation: "open process", Message: fmt.Sprintf("server denied or could not open PID %d", pid)}
	}
	return handle, nil
}

func (client *Client) CloseHandle(handle uint32) error {
	packet := bytes.NewBuffer(make([]byte, 0, 5))
	packet.WriteByte(commandCloseHandle)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return client.protocolError("close handle", err)
	}
	return nil
}

func (client *Client) Architecture(handle uint32) (Architecture, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 5))
	packet.WriteByte(commandGetArchitecture)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	response := []byte{0}
	if err := client.readFull(response); err != nil {
		return 0, client.protocolError("get architecture", err)
	}
	return Architecture(response[0]), nil
}

func (client *Client) ListProcesses() ([]Process, error) {
	handle, err := client.createSnapshot(toolhelpSnapshotProcess, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.CloseHandle(handle) }()

	processes := make([]Process, 0, 64)
	command := commandProcess32First
	for len(processes) < maximumCollectionEntries {
		packet := bytes.NewBuffer(make([]byte, 0, 5))
		packet.WriteByte(command)
		_ = binary.Write(packet, binary.LittleEndian, handle)
		if err := client.writePacket(packet.Bytes()); err != nil {
			return nil, err
		}
		var result int32
		var pid int32
		var nameLength int32
		if err := client.readValue(&result); err != nil {
			return nil, client.protocolError("list processes", err)
		}
		if err := client.readValue(&pid); err != nil {
			return nil, client.protocolError("list processes", err)
		}
		if err := client.readValue(&nameLength); err != nil {
			return nil, client.protocolError("list processes", err)
		}
		if result == 0 {
			break
		}
		name, err := client.readString(int64(nameLength))
		if err != nil {
			return nil, client.protocolError("read process name", err)
		}
		processes = append(processes, Process{PID: pid, Name: name})
		command = commandProcess32Next
	}
	return processes, nil
}

func (client *Client) ListModules(pid int32) ([]Module, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(commandCreateToolhelp32SnapshotEx)
	_ = binary.Write(packet, binary.LittleEndian, toolhelpSnapshotModule)
	_ = binary.Write(packet, binary.LittleEndian, uint32(pid))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}

	modules := make([]Module, 0, 32)
	for len(modules) < maximumCollectionEntries {
		header := make([]byte, 28)
		if err := client.readFull(header); err != nil {
			return nil, client.protocolError("list modules", err)
		}
		result := int32(binary.LittleEndian.Uint32(header[0:4]))
		nameLength := int64(int32(binary.LittleEndian.Uint32(header[24:28])))
		if result == 0 {
			break
		}
		name, err := client.readString(nameLength)
		if err != nil {
			return nil, client.protocolError("read module name", err)
		}
		modules = append(modules, Module{
			BaseAddress: binary.LittleEndian.Uint64(header[4:12]),
			Part:        int32(binary.LittleEndian.Uint32(header[12:16])),
			Size:        binary.LittleEndian.Uint32(header[16:20]),
			FileOffset:  binary.LittleEndian.Uint32(header[20:24]),
			Name:        name,
		})
	}
	return modules, nil
}

func (client *Client) ListThreads(pid int32) ([]Thread, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(commandCreateToolhelp32SnapshotEx)
	_ = binary.Write(packet, binary.LittleEndian, toolhelpSnapshotThread)
	_ = binary.Write(packet, binary.LittleEndian, uint32(pid))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var count int32
	if err := client.readValue(&count); err != nil {
		return nil, client.protocolError("list threads", err)
	}
	if count < 0 || count > maximumCollectionEntries {
		return nil, &ProtocolError{Operation: "list threads", Message: fmt.Sprintf("invalid thread count %d", count)}
	}
	threads := make([]Thread, count)
	for index := range threads {
		if err := client.readValue(&threads[index].TID); err != nil {
			return nil, client.protocolError("list threads", err)
		}
	}
	return threads, nil
}

func (client *Client) ProcessInfo(pid int32) (ProcessInfo, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return ProcessInfo{}, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	architecture, err := client.Architecture(handle)
	if err != nil {
		return ProcessInfo{}, err
	}
	modules, err := client.ListModules(pid)
	if err != nil {
		return ProcessInfo{}, err
	}
	threads, err := client.ListThreads(pid)
	if err != nil {
		return ProcessInfo{}, err
	}
	return ProcessInfo{PID: pid, Architecture: architecture.String(), ModuleCount: len(modules), ThreadCount: len(threads)}, nil
}

func (client *Client) ReadMemory(pid int32, address uint64, size uint32) ([]byte, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	return client.readMemoryWithHandle(handle, address, size)
}

func (client *Client) readMemoryWithHandle(handle uint32, address uint64, size uint32) ([]byte, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 18))
	packet.WriteByte(commandReadProcessMemory)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, address)
	_ = binary.Write(packet, binary.LittleEndian, size)
	packet.WriteByte(0)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var bytesRead int32
	if err := client.readValue(&bytesRead); err != nil {
		return nil, client.protocolError("read memory", err)
	}
	if bytesRead < 0 || uint32(bytesRead) > size {
		return nil, &ProtocolError{Operation: "read memory", Message: fmt.Sprintf("invalid byte count %d", bytesRead)}
	}
	data := make([]byte, bytesRead)
	if err := client.readFull(data); err != nil {
		return nil, client.protocolError("read memory bytes", err)
	}
	return data, nil
}

func (client *Client) WriteMemory(pid int32, address uint64, data []byte) (int32, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 17+len(data)))
	packet.WriteByte(commandWriteProcessMemory)
	_ = binary.Write(packet, binary.LittleEndian, int32(handle))
	_ = binary.Write(packet, binary.LittleEndian, int64(address))
	_ = binary.Write(packet, binary.LittleEndian, int32(len(data)))
	packet.Write(data)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var written int32
	if err := client.readValue(&written); err != nil {
		return 0, client.protocolError("write memory", err)
	}
	return written, nil
}

func (client *Client) Regions(pid int32, flags RegionQueryFlags) ([]Region, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	return client.regionsWithHandle(handle, flags)
}

func (client *Client) regionsWithHandle(handle uint32, flags RegionQueryFlags) ([]Region, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 6))
	packet.WriteByte(commandVirtualQueryExFull)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	packet.WriteByte(byte(flags))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var count uint32
	if err := client.readValue(&count); err != nil {
		return nil, client.protocolError("list memory regions", err)
	}
	if count > maximumCollectionEntries {
		return nil, &ProtocolError{Operation: "list memory regions", Message: fmt.Sprintf("invalid region count %d", count)}
	}
	regions := make([]Region, count)
	for index := range regions {
		header := make([]byte, 24)
		if err := client.readFull(header); err != nil {
			return nil, client.protocolError("list memory regions", err)
		}
		protection := Protection(binary.LittleEndian.Uint32(header[16:20]))
		memoryType := MemoryType(binary.LittleEndian.Uint32(header[20:24]))
		regions[index] = Region{
			BaseAddress: binary.LittleEndian.Uint64(header[0:8]),
			Size:        binary.LittleEndian.Uint64(header[8:16]),
			Protection:  protection,
			Permissions: protection.String(),
			Type:        memoryType,
			TypeName:    memoryType.String(),
		}
	}
	return regions, nil
}

func (client *Client) ScanMemory(ctx context.Context, pid int32, pattern, mask []byte, start, end uint64, alignment int, protection Protection, maximumMatches int) ([]uint64, error) {
	if len(pattern) == 0 || len(pattern) != len(mask) {
		return nil, errors.New("pattern and mask must be non-empty and have equal lengths")
	}
	if alignment < 1 {
		return nil, errors.New("alignment must be at least 1")
	}
	if end <= start {
		return nil, errors.New("scan end must be greater than scan start")
	}
	if maximumMatches < 1 || maximumMatches > maximumCollectionEntries {
		return nil, fmt.Errorf("maximum matches must be between 1 and %d", maximumCollectionEntries)
	}
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	regions, err := client.regionsWithHandle(handle, 0)
	if err != nil {
		return nil, err
	}

	const chunkSize uint64 = 1 << 20
	matches := make([]uint64, 0, min(maximumMatches, 1024))
	for _, region := range regions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if uint32(region.Protection)&uint32(protection) == 0 {
			continue
		}
		regionStart := max(region.BaseAddress, start)
		regionEnd := min(saturatingAdd(region.BaseAddress, region.Size), end)
		if regionEnd <= regionStart || regionEnd-regionStart < uint64(len(pattern)) {
			continue
		}
		var tail []byte
		nextCandidate := regionStart
		for address := regionStart; address < regionEnd; {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			requested := min(chunkSize, regionEnd-address)
			data, readErr := client.readMemoryWithHandle(handle, address, uint32(requested))
			if readErr != nil || len(data) == 0 {
				break
			}
			combined := make([]byte, 0, len(tail)+len(data))
			combined = append(combined, tail...)
			combined = append(combined, data...)
			baseAddress := address - uint64(len(tail))
			matches = scanBuffer(matches, combined, pattern, mask, baseAddress, start, alignment, nextCandidate, maximumMatches)
			if len(matches) >= maximumMatches {
				return matches, nil
			}
			if len(combined) >= len(pattern) {
				nextCandidate = baseAddress + uint64(len(combined)-len(pattern)+1)
			}
			tailLength := min(len(pattern)-1, len(combined))
			tail = append(tail[:0], combined[len(combined)-tailLength:]...)
			address += uint64(len(data))
			if uint64(len(data)) < requested {
				break
			}
		}
	}
	return matches, nil
}

func (client *Client) AOBScan(pid int32, pattern, mask []byte, start, end uint64, alignment int32, protection Protection) ([]uint64, error) {
	if len(pattern) == 0 || len(pattern) != len(mask) {
		return nil, errors.New("pattern and mask must be non-empty and have equal lengths")
	}
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 33+len(pattern)*2))
	packet.WriteByte(commandAOBScan)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, start)
	_ = binary.Write(packet, binary.LittleEndian, end)
	_ = binary.Write(packet, binary.LittleEndian, alignment)
	_ = binary.Write(packet, binary.LittleEndian, int32(protection))
	_ = binary.Write(packet, binary.LittleEndian, int32(len(pattern)))
	packet.Write(pattern)
	packet.Write(mask)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var count int32
	if err := client.readValue(&count); err != nil {
		return nil, client.protocolError("AOB scan", err)
	}
	if count < 0 {
		return nil, &ProtocolError{Operation: "AOB scan", Message: "server could not scan the requested address range"}
	}
	if count > maximumCollectionEntries {
		return nil, &ProtocolError{Operation: "AOB scan", Message: fmt.Sprintf("invalid match count %d", count)}
	}
	addresses := make([]uint64, count)
	for index := range addresses {
		if err := client.readValue(&addresses[index]); err != nil {
			return nil, client.protocolError("read AOB scan matches", err)
		}
	}
	return addresses, nil
}

func (client *Client) createSnapshot(flags uint32, pid uint32) (uint32, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(commandCreateToolhelp32Snapshot)
	_ = binary.Write(packet, binary.LittleEndian, flags)
	_ = binary.Write(packet, binary.LittleEndian, pid)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var handle uint32
	if err := client.readValue(&handle); err != nil {
		return 0, client.protocolError("create snapshot", err)
	}
	if handle == 0 {
		return 0, &ProtocolError{Operation: "create snapshot", Message: "server returned an invalid snapshot handle"}
	}
	return handle, nil
}

func (client *Client) writePacket(packet []byte) error {
	if err := client.setDeadline(); err != nil {
		return client.protocolError("set deadline", err)
	}
	for len(packet) > 0 {
		written, err := client.connection.Write(packet)
		if err != nil {
			return client.protocolError("send request", err)
		}
		packet = packet[written:]
	}
	return nil
}

func (client *Client) readFull(destination []byte) error {
	if len(destination) == 0 {
		return nil
	}
	if err := client.setDeadline(); err != nil {
		return err
	}
	_, err := io.ReadFull(client.connection, destination)
	return err
}

func (client *Client) readValue(destination any) error {
	if err := client.setDeadline(); err != nil {
		return err
	}
	return binary.Read(client.connection, binary.LittleEndian, destination)
}

func (client *Client) readString(length int64) (string, error) {
	if length < 0 || length > maximumProtocolStringLength {
		return "", fmt.Errorf("invalid string length %d", length)
	}
	value := make([]byte, length)
	if err := client.readFull(value); err != nil {
		return "", err
	}
	return string(value), nil
}

func (client *Client) readString16() (string, error) {
	var length uint16
	if err := client.readValue(&length); err != nil {
		return "", err
	}
	return client.readString(int64(length))
}

func writeString16(packet *bytes.Buffer, value string) error {
	if len(value) > int(^uint16(0)) {
		return fmt.Errorf("string exceeds %d bytes", ^uint16(0))
	}
	if bytes.IndexByte([]byte(value), 0) >= 0 {
		return errors.New("string contains a NUL byte")
	}
	if err := binary.Write(packet, binary.LittleEndian, uint16(len(value))); err != nil {
		return err
	}
	_, err := packet.WriteString(value)
	return err
}

func (client *Client) setDeadline() error {
	return client.connection.SetDeadline(time.Now().Add(client.timeout))
}

func (client *Client) protocolError(operation string, err error) error {
	return &ProtocolError{Operation: operation, Message: err.Error()}
}

func scanBuffer(matches []uint64, data, pattern, mask []byte, baseAddress, scanStart uint64, alignment int, minimumCandidate uint64, maximumMatches int) []uint64 {
	if len(data) < len(pattern) {
		return matches
	}
	for offset := 0; offset <= len(data)-len(pattern); offset++ {
		address := baseAddress + uint64(offset)
		if address < minimumCandidate || (address-scanStart)%uint64(alignment) != 0 {
			continue
		}
		matched := true
		for patternIndex := range pattern {
			if mask[patternIndex] != '?' && data[offset+patternIndex] != pattern[patternIndex] {
				matched = false
				break
			}
		}
		if matched {
			matches = append(matches, address)
			if len(matches) >= maximumMatches {
				return matches
			}
		}
	}
	return matches
}

func saturatingAdd(left, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}
