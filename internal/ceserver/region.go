package ceserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func (client *Client) RegionInfo(pid int32, address uint64) (RegionDetail, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return RegionDetail{}, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 13))
	packet.WriteByte(commandGetRegionInfo)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, address)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return RegionDetail{}, err
	}
	header := make([]byte, 25)
	if err := client.readFull(header); err != nil {
		return RegionDetail{}, client.protocolError("get region info", err)
	}
	result := header[0]
	protection := Protection(binary.LittleEndian.Uint32(header[1:5]))
	memoryType := MemoryType(binary.LittleEndian.Uint32(header[5:9]))
	baseAddress := binary.LittleEndian.Uint64(header[9:17])
	size := binary.LittleEndian.Uint64(header[17:25])
	mapLength := []byte{0}
	if err := client.readFull(mapLength); err != nil {
		return RegionDetail{}, client.protocolError("get region map line", err)
	}
	mapsLine, err := client.readString(int64(mapLength[0]))
	if err != nil {
		return RegionDetail{}, client.protocolError("get region map line", err)
	}
	if result == 0 {
		return RegionDetail{}, &ProtocolError{Operation: "get region info", Message: fmt.Sprintf("address 0x%X was not mapped", address)}
	}
	return RegionDetail{
		Region:   Region{BaseAddress: baseAddress, Size: size, Protection: protection, Permissions: protection.String(), Type: memoryType, TypeName: memoryType.String()},
		MapsLine: mapsLine,
	}, nil
}
