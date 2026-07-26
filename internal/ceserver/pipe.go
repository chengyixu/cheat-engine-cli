package ceserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func (client *Client) OpenPipe(name string, timeoutMilliseconds uint32) (uint32, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 7+len(name)))
	packet.WriteByte(commandOpenNamedPipe)
	if err := writeString16(packet, name); err != nil {
		return 0, err
	}
	_ = binary.Write(packet, binary.LittleEndian, timeoutMilliseconds)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var handle uint32
	if err := client.readValue(&handle); err != nil {
		return 0, client.protocolError("open named pipe", err)
	}
	if handle == 0 {
		return 0, &ProtocolError{Operation: "open named pipe", Message: fmt.Sprintf("pipe %q was unavailable", name)}
	}
	return handle, nil
}

func (client *Client) ReadPipe(handle, size, timeoutMilliseconds uint32) ([]byte, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 13))
	packet.WriteByte(commandPipeRead)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, size)
	_ = binary.Write(packet, binary.LittleEndian, timeoutMilliseconds)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var bytesRead int32
	if err := client.readValue(&bytesRead); err != nil {
		return nil, client.protocolError("read named pipe", err)
	}
	if bytesRead < 0 || uint32(bytesRead) > size {
		return nil, &ProtocolError{Operation: "read named pipe", Message: fmt.Sprintf("invalid byte count %d", bytesRead)}
	}
	data := make([]byte, bytesRead)
	if err := client.readFull(data); err != nil {
		return nil, client.protocolError("read named pipe data", err)
	}
	return data, nil
}

func (client *Client) WritePipe(handle uint32, data []byte, timeoutMilliseconds uint32) (int32, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 13+len(data)))
	packet.WriteByte(commandPipeWrite)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, uint32(len(data)))
	_ = binary.Write(packet, binary.LittleEndian, timeoutMilliseconds)
	packet.Write(data)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var written int32
	if err := client.readValue(&written); err != nil {
		return 0, client.protocolError("write named pipe", err)
	}
	return written, nil
}
