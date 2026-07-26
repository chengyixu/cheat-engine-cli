package ceserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

func (client *Client) AllocateMemory(pid int32, preferredAddress uint64, size uint32, protection Protection) (uint64, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 21))
	packet.WriteByte(commandAllocateMemory)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, preferredAddress)
	_ = binary.Write(packet, binary.LittleEndian, size)
	_ = binary.Write(packet, binary.LittleEndian, uint32(protection))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var address uint64
	if err := client.readValue(&address); err != nil {
		return 0, client.protocolError("allocate memory", err)
	}
	if address == 0 {
		return 0, &ProtocolError{Operation: "allocate memory", Message: "server could not allocate the requested region"}
	}
	return address, nil
}

func (client *Client) FreeMemory(pid int32, address uint64, size uint32) (bool, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return false, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 17))
	packet.WriteByte(commandFreeMemory)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, address)
	_ = binary.Write(packet, binary.LittleEndian, size)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	var result uint32
	if err := client.readValue(&result); err != nil {
		return false, client.protocolError("free memory", err)
	}
	return result != 0, nil
}

func (client *Client) ChangeMemoryProtection(pid int32, address uint64, size uint32, protection Protection) (ProtectionChange, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return ProtectionChange{}, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 21))
	packet.WriteByte(commandChangeMemoryProtection)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, address)
	_ = binary.Write(packet, binary.LittleEndian, size)
	_ = binary.Write(packet, binary.LittleEndian, uint32(protection))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return ProtectionChange{}, err
	}
	var result uint32
	var oldProtection Protection
	if err := client.readValue(&result); err != nil {
		return ProtectionChange{}, client.protocolError("change memory protection", err)
	}
	if err := client.readValue(&oldProtection); err != nil {
		return ProtectionChange{}, client.protocolError("change memory protection", err)
	}
	change := ProtectionChange{
		Address: address, Size: size, OldProtection: oldProtection, OldPermissions: oldProtection.String(),
		NewProtection: protection, NewPermissions: protection.String(),
	}
	if result != 0 {
		return change, &ProtocolError{Operation: "change memory protection", Message: fmt.Sprintf("server returned failure code %d", result)}
	}
	return change, nil
}

func (client *Client) SetSpeed(pid int32, speed float32) (bool, error) {
	if math.IsNaN(float64(speed)) || math.IsInf(float64(speed), 0) || speed <= 0 {
		return false, fmt.Errorf("speed must be a finite positive number")
	}
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return false, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(commandSetSpeed)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, speed)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	var result uint32
	if err := client.readValue(&result); err != nil {
		return false, client.protocolError("set process speed", err)
	}
	return result != 0, nil
}

func (client *Client) LoadModule(pid int32, remotePath string) (uint64, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	if len(remotePath) > maximumProtocolStringLength {
		return 0, fmt.Errorf("module path exceeds protocol limit")
	}
	packet := bytes.NewBuffer(make([]byte, 0, 9+len(remotePath)))
	packet.WriteByte(commandLoadModule)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, uint32(len(remotePath)))
	packet.WriteString(remotePath)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var address uint64
	if err := client.readValue(&address); err != nil {
		return 0, client.protocolError("load module", err)
	}
	if address == 0 {
		return 0, &ProtocolError{Operation: "load module", Message: "server could not load the requested module"}
	}
	return address, nil
}

func (client *Client) LoadModuleEx(pid int32, dlopenAddress uint64, remotePath string) (uint64, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	if len(remotePath) == 0 || len(remotePath) > maximumProtocolStringLength {
		return 0, fmt.Errorf("module path must be between 1 and %d bytes", maximumProtocolStringLength)
	}
	packet := bytes.NewBuffer(make([]byte, 0, 17+len(remotePath)))
	packet.WriteByte(commandLoadModuleEx)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, dlopenAddress)
	_ = binary.Write(packet, binary.LittleEndian, uint32(len(remotePath)))
	packet.WriteString(remotePath)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var address uint64
	if err := client.readValue(&address); err != nil {
		return 0, client.protocolError("load module with explicit dlopen", err)
	}
	if address == 0 {
		return 0, &ProtocolError{Operation: "load module with explicit dlopen", Message: "server could not load the requested module"}
	}
	return address, nil
}

func (client *Client) LoadExtension(pid int32) (bool, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return false, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 5))
	packet.WriteByte(commandLoadExtension)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return false, client.protocolError("load ceserver extension", err)
	}
	return result != 0, nil
}

func (client *Client) CreateRemoteThread(pid int32, startAddress, parameter uint64) (uint32, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 21))
	packet.WriteByte(commandCreateRemoteThread)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, startAddress)
	_ = binary.Write(packet, binary.LittleEndian, parameter)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var threadHandle uint32
	if err := client.readValue(&threadHandle); err != nil {
		return 0, client.protocolError("create remote thread", err)
	}
	if threadHandle == 0 {
		return 0, &ProtocolError{Operation: "create remote thread", Message: "server could not create the requested thread"}
	}
	return threadHandle, nil
}
