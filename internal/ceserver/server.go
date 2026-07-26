package ceserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

func (client *Client) SetConnectionName(name string) error {
	if name == "" {
		return errors.New("connection name cannot be empty")
	}
	if len(name) > maximumProtocolStringLength {
		return fmt.Errorf("connection name exceeds %d bytes", maximumProtocolStringLength)
	}
	if bytes.IndexByte([]byte(name), 0) >= 0 {
		return errors.New("connection name contains a NUL byte")
	}
	packet := bytes.NewBuffer(make([]byte, 0, 5+len(name)))
	packet.WriteByte(commandSetConnectionName)
	_ = binary.Write(packet, binary.LittleEndian, uint32(len(name)))
	packet.WriteString(name)
	return client.writePacket(packet.Bytes())
}

func (client *Client) TerminateServer() error {
	if err := client.writePacket([]byte{commandTerminateServer}); err != nil {
		return err
	}
	connection := client.connection
	client.connection = nil
	return connection.Close()
}

func (client *Client) PathInfo() (ServerPathInfo, error) {
	executablePath, err := client.ServerPath()
	if err != nil {
		return ServerPathInfo{}, err
	}
	currentPath, err := client.CurrentPath()
	if err != nil {
		return ServerPathInfo{}, err
	}
	android, err := client.IsAndroid()
	if err != nil {
		return ServerPathInfo{}, err
	}
	return ServerPathInfo{ExecutablePath: executablePath, CurrentPath: currentPath, Android: android}, nil
}

func (client *Client) ServerPath() (string, error) {
	if err := client.writePacket([]byte{commandGetServerPath}); err != nil {
		return "", err
	}
	value, err := client.readString16()
	if err != nil {
		return "", client.protocolError("get server path", err)
	}
	return value, nil
}

func (client *Client) IsAndroid() (bool, error) {
	if err := client.writePacket([]byte{commandIsAndroid}); err != nil {
		return false, err
	}
	response := []byte{0}
	if err := client.readFull(response); err != nil {
		return false, client.protocolError("detect Android server", err)
	}
	return response[0] != 0, nil
}

func (client *Client) CurrentPath() (string, error) {
	if err := client.writePacket([]byte{commandGetCurrentPath}); err != nil {
		return "", err
	}
	value, err := client.readString16()
	if err != nil {
		return "", client.protocolError("get current path", err)
	}
	return value, nil
}

func (client *Client) SetCurrentPath(path string) (bool, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(path)))
	packet.WriteByte(commandSetCurrentPath)
	if err := writeString16(packet, path); err != nil {
		return false, err
	}
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	response := []byte{0}
	if err := client.readFull(response); err != nil {
		return false, client.protocolError("set current path", err)
	}
	return response[0] != 0, nil
}

func (client *Client) Options() ([]ServerOption, error) {
	if err := client.writePacket([]byte{commandGetOptions}); err != nil {
		return nil, err
	}
	var count uint16
	if err := client.readValue(&count); err != nil {
		return nil, client.protocolError("get server options", err)
	}
	if count > 4096 {
		return nil, &ProtocolError{Operation: "get server options", Message: fmt.Sprintf("invalid option count %d", count)}
	}
	options := make([]ServerOption, count)
	for index := range options {
		fields := make([]string, 5)
		for fieldIndex := range fields {
			value, err := client.readString16()
			if err != nil {
				return nil, client.protocolError("get server options", err)
			}
			fields[fieldIndex] = value
		}
		var typeCode int32
		if err := client.readValue(&typeCode); err != nil {
			return nil, client.protocolError("get server option type", err)
		}
		options[index] = ServerOption{
			Name: fields[0], Parent: fields[1], Description: fields[2],
			AcceptableValues: fields[3], CurrentValue: fields[4], TypeCode: typeCode, Type: optionTypeName(typeCode),
		}
	}
	return options, nil
}

func (client *Client) Option(name string) (string, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 3+len(name)))
	packet.WriteByte(commandGetOptionValue)
	if err := writeString16(packet, name); err != nil {
		return "", err
	}
	if err := client.writePacket(packet.Bytes()); err != nil {
		return "", err
	}
	value, err := client.readString16()
	if err != nil {
		return "", client.protocolError("get server option", err)
	}
	return value, nil
}

func (client *Client) SetOption(name, value string) error {
	packet := bytes.NewBuffer(make([]byte, 0, 5+len(name)+len(value)))
	packet.WriteByte(commandSetOptionValue)
	if err := writeString16(packet, name); err != nil {
		return err
	}
	if err := writeString16(packet, value); err != nil {
		return err
	}
	return client.writePacket(packet.Bytes())
}

func optionTypeName(typeCode int32) string {
	switch typeCode {
	case 0:
		return "parent"
	case 1:
		return "boolean"
	case 2:
		return "integer"
	case 3:
		return "float"
	case 4:
		return "double"
	case 5:
		return "text"
	default:
		return "unknown"
	}
}
