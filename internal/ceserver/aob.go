package ceserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

func (client *Client) ServerAOBScan(pid int32, pattern, mask []byte, start, end uint64, alignment int, protection Protection) ([]uint64, error) {
	if len(pattern) == 0 || len(pattern) != len(mask) {
		return nil, errors.New("pattern and mask must be non-empty and have equal lengths")
	}
	if len(pattern) > maximumServerAOBPatternSize {
		return nil, fmt.Errorf("pattern exceeds %d bytes", maximumServerAOBPatternSize)
	}
	if alignment < 1 || uint64(alignment) > uint64(^uint32(0)>>1) {
		return nil, errors.New("alignment must fit a positive 32-bit integer")
	}
	if end <= start {
		return nil, errors.New("scan end must be greater than scan start")
	}
	for _, value := range mask {
		if value != 'x' && value != '?' {
			return nil, errors.New("mask may contain only 'x' and '?' bytes")
		}
	}
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.CloseHandle(handle) }()

	packet := bytes.NewBuffer(make([]byte, 0, 33+2*len(pattern)))
	packet.WriteByte(commandAOBScan)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, start)
	_ = binary.Write(packet, binary.LittleEndian, end)
	_ = binary.Write(packet, binary.LittleEndian, int32(alignment))
	_ = binary.Write(packet, binary.LittleEndian, uint32(protection))
	_ = binary.Write(packet, binary.LittleEndian, int32(len(pattern)))
	packet.Write(pattern)
	packet.Write(mask)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var count int32
	if err := client.readValue(&count); err != nil {
		return nil, client.protocolError("server AOB scan", err)
	}
	if count < 0 || count > maximumCollectionEntries {
		return nil, &ProtocolError{Operation: "server AOB scan", Message: fmt.Sprintf("invalid match count %d", count)}
	}
	matches := make([]uint64, count)
	for index := range matches {
		if err := client.readValue(&matches[index]); err != nil {
			return nil, client.protocolError("server AOB scan matches", err)
		}
	}
	return matches, nil
}
