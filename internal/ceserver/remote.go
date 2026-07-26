package ceserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func (client *Client) ListRemoteFiles(path string) ([]RemoteFile, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(path)))
	packet.WriteByte(commandEnumerateFiles)
	if err := writeString16(packet, path); err != nil {
		return nil, err
	}
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	files := make([]RemoteFile, 0, 32)
	for len(files) < maximumCollectionEntries {
		name, err := client.readString16()
		if err != nil {
			return nil, client.protocolError("list remote files", err)
		}
		if name == "" {
			break
		}
		typeResponse := []byte{0}
		if err := client.readFull(typeResponse); err != nil {
			return nil, client.protocolError("read remote file type", err)
		}
		files = append(files, RemoteFile{Name: name, TypeCode: typeResponse[0], Type: remoteFileTypeName(typeResponse[0])})
	}
	return files, nil
}

func (client *Client) RemoteFilePermissions(path string) (uint32, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(path)))
	packet.WriteByte(commandGetFilePermissions)
	if err := writeString16(packet, path); err != nil {
		return 0, err
	}
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	response := []byte{0}
	if err := client.readFull(response); err != nil {
		return 0, client.protocolError("get remote file permissions", err)
	}
	if response[0] == 0 {
		return 0, &ProtocolError{Operation: "get remote file permissions", Message: fmt.Sprintf("path %q was not found or could not be inspected", path)}
	}
	var permissions uint32
	if err := client.readValue(&permissions); err != nil {
		return 0, client.protocolError("get remote file permissions", err)
	}
	return permissions, nil
}

func (client *Client) SetRemoteFilePermissions(path string, permissions uint32) (bool, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 7+len(path)))
	packet.WriteByte(commandSetFilePermissions)
	if err := writeString16(packet, path); err != nil {
		return false, err
	}
	if err := binary.Write(packet, binary.LittleEndian, permissions); err != nil {
		return false, err
	}
	return client.remoteBooleanOperation("set remote file permissions", packet.Bytes())
}

func (client *Client) GetRemoteFile(path string) ([]byte, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(path)))
	packet.WriteByte(commandGetFile)
	if err := writeString16(packet, path); err != nil {
		return nil, err
	}
	if err := client.writePacket(packet.Bytes()); err != nil {
		return nil, err
	}
	var size uint32
	if err := client.readValue(&size); err != nil {
		return nil, client.protocolError("get remote file", err)
	}
	if size == ^uint32(0) {
		return nil, &ProtocolError{Operation: "get remote file", Message: fmt.Sprintf("path %q was not found or could not be read", path)}
	}
	if size > maximumRemoteFileSize {
		return nil, &ProtocolError{Operation: "get remote file", Message: fmt.Sprintf("remote file size %d exceeds client limit %d", size, maximumRemoteFileSize)}
	}
	content := make([]byte, size)
	if err := client.readFull(content); err != nil {
		return nil, client.protocolError("get remote file content", err)
	}
	return content, nil
}

func (client *Client) PutRemoteFile(path string, content []byte) (bool, error) {
	if len(content) > maximumRemoteFileSize {
		return false, fmt.Errorf("local file size %d exceeds client limit %d", len(content), maximumRemoteFileSize)
	}
	packet := bytes.NewBuffer(make([]byte, 0, 7+len(path)+len(content)))
	packet.WriteByte(commandPutFile)
	if err := writeString16(packet, path); err != nil {
		return false, err
	}
	if err := binary.Write(packet, binary.LittleEndian, uint32(len(content))); err != nil {
		return false, err
	}
	packet.Write(content)
	return client.remoteBooleanOperation("put remote file", packet.Bytes())
}

func (client *Client) CreateRemoteDirectory(path string) (bool, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(path)))
	packet.WriteByte(commandCreateDirectory)
	if err := writeString16(packet, path); err != nil {
		return false, err
	}
	return client.remoteBooleanOperation("create remote directory", packet.Bytes())
}

func (client *Client) DeleteRemotePath(path string) (bool, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(path)))
	packet.WriteByte(commandDeleteFile)
	if err := writeString16(packet, path); err != nil {
		return false, err
	}
	return client.remoteBooleanOperation("delete remote path", packet.Bytes())
}

func (client *Client) remoteBooleanOperation(operation string, packet []byte) (bool, error) {
	if err := client.writePacket(packet); err != nil {
		return false, err
	}
	response := []byte{0}
	if err := client.readFull(response); err != nil {
		return false, client.protocolError(operation, err)
	}
	return response[0] != 0, nil
}

func remoteFileTypeName(typeCode uint8) string {
	switch typeCode {
	case 4:
		return "directory"
	case 8:
		return "file"
	case 10:
		return "symlink"
	default:
		return "unknown"
	}
}
