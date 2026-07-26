package ceserver

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

func (client *Client) Symbols(path string, fileOffset uint32) (SymbolList, error) {
	if len(path) == 0 || len(path) > maximumProtocolStringLength {
		return SymbolList{}, fmt.Errorf("symbol path must be between 1 and %d bytes", maximumProtocolStringLength)
	}
	packet := bytes.NewBuffer(make([]byte, 0, 9+len(path)))
	packet.WriteByte(commandGetSymbols)
	_ = binary.Write(packet, binary.LittleEndian, fileOffset)
	_ = binary.Write(packet, binary.LittleEndian, uint32(len(path)))
	packet.WriteString(path)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return SymbolList{}, err
	}
	var executable uint32
	var totalSize uint32
	if err := client.readValue(&executable); err != nil {
		return SymbolList{}, client.protocolError("get symbols", err)
	}
	if err := client.readValue(&totalSize); err != nil {
		return SymbolList{}, client.protocolError("get symbols", err)
	}
	if totalSize == 0 {
		return SymbolList{}, &ProtocolError{Operation: "get symbols", Message: fmt.Sprintf("no symbols were returned for %q", path)}
	}
	if totalSize < 12 || totalSize > maximumRemoteFileSize {
		return SymbolList{}, &ProtocolError{Operation: "get symbols", Message: fmt.Sprintf("invalid compressed response size %d", totalSize)}
	}
	var decompressedSize uint32
	if err := client.readValue(&decompressedSize); err != nil {
		return SymbolList{}, client.protocolError("get symbols", err)
	}
	if decompressedSize > maximumRemoteFileSize {
		return SymbolList{}, &ProtocolError{Operation: "get symbols", Message: fmt.Sprintf("symbol response exceeds %d bytes", maximumRemoteFileSize)}
	}
	compressed := make([]byte, totalSize-12)
	if err := client.readFull(compressed); err != nil {
		return SymbolList{}, client.protocolError("get compressed symbols", err)
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return SymbolList{}, client.protocolError("decompress symbols", err)
	}
	decompressed, err := io.ReadAll(io.LimitReader(reader, int64(decompressedSize)+1))
	closeErr := reader.Close()
	if err != nil {
		return SymbolList{}, client.protocolError("decompress symbols", err)
	}
	if closeErr != nil {
		return SymbolList{}, client.protocolError("close symbol stream", closeErr)
	}
	if len(decompressed) != int(decompressedSize) {
		return SymbolList{}, &ProtocolError{Operation: "get symbols", Message: fmt.Sprintf("decompressed %d bytes, expected %d", len(decompressed), decompressedSize)}
	}
	symbols, err := parseSymbols(decompressed)
	if err != nil {
		return SymbolList{}, err
	}
	return SymbolList{Path: path, FileOffset: fileOffset, Executable: executable != 0, Symbols: symbols, SymbolCount: len(symbols)}, nil
}

func parseSymbols(data []byte) ([]Symbol, error) {
	symbols := make([]Symbol, 0, 1024)
	for position := 0; position < len(data); {
		if len(data)-position < 17 {
			return nil, &ProtocolError{Operation: "parse symbols", Message: fmt.Sprintf("truncated symbol header at byte %d", position)}
		}
		address := binary.LittleEndian.Uint64(data[position : position+8])
		size := int32(binary.LittleEndian.Uint32(data[position+8 : position+12]))
		typeCode := int32(binary.LittleEndian.Uint32(data[position+12 : position+16]))
		nameLength := int(data[position+16])
		position += 17
		if len(data)-position < nameLength {
			return nil, &ProtocolError{Operation: "parse symbols", Message: fmt.Sprintf("truncated symbol name at byte %d", position)}
		}
		name := string(data[position : position+nameLength])
		position += nameLength
		if name != "" {
			symbols = append(symbols, Symbol{Address: address, Size: size, Type: typeCode, Name: name})
		}
		if len(symbols) > maximumCollectionEntries {
			return nil, &ProtocolError{Operation: "parse symbols", Message: "symbol count exceeds client limit"}
		}
	}
	return symbols, nil
}
