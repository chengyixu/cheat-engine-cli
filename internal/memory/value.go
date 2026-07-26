package memory

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

var supportedTypes = []string{"u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64", "f32", "f64", "utf8", "utf16le", "hex"}

func SupportedTypes() []string {
	return append([]string(nil), supportedTypes...)
}

func ParseAddress(value string) (uint64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "_", "")
	if normalized == "" {
		return 0, fmt.Errorf("address is empty")
	}
	address, err := strconv.ParseUint(normalized, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid address %q: %w", value, err)
	}
	return address, nil
}

func Encode(valueType, value string) ([]byte, error) {
	valueType = strings.ToLower(strings.TrimSpace(valueType))
	switch valueType {
	case "u8":
		parsed, err := strconv.ParseUint(value, 0, 8)
		return []byte{byte(parsed)}, wrapParse(err, valueType, value)
	case "i8":
		parsed, err := strconv.ParseInt(value, 0, 8)
		return []byte{byte(int8(parsed))}, wrapParse(err, valueType, value)
	case "u16":
		parsed, err := strconv.ParseUint(value, 0, 16)
		return encodeUint16(uint16(parsed)), wrapParse(err, valueType, value)
	case "i16":
		parsed, err := strconv.ParseInt(value, 0, 16)
		return encodeUint16(uint16(int16(parsed))), wrapParse(err, valueType, value)
	case "u32":
		parsed, err := strconv.ParseUint(value, 0, 32)
		return encodeUint32(uint32(parsed)), wrapParse(err, valueType, value)
	case "i32":
		parsed, err := strconv.ParseInt(value, 0, 32)
		return encodeUint32(uint32(int32(parsed))), wrapParse(err, valueType, value)
	case "u64":
		parsed, err := strconv.ParseUint(value, 0, 64)
		return encodeUint64(parsed), wrapParse(err, valueType, value)
	case "i64":
		parsed, err := strconv.ParseInt(value, 0, 64)
		return encodeUint64(uint64(parsed)), wrapParse(err, valueType, value)
	case "f32":
		parsed, err := strconv.ParseFloat(value, 32)
		return encodeUint32(math.Float32bits(float32(parsed))), wrapParse(err, valueType, value)
	case "f64":
		parsed, err := strconv.ParseFloat(value, 64)
		return encodeUint64(math.Float64bits(parsed)), wrapParse(err, valueType, value)
	case "utf8":
		return []byte(value), nil
	case "utf16le":
		codeUnits := utf16.Encode([]rune(value))
		encoded := make([]byte, len(codeUnits)*2)
		for index, codeUnit := range codeUnits {
			binary.LittleEndian.PutUint16(encoded[index*2:], codeUnit)
		}
		return encoded, nil
	case "hex":
		return ParseHex(value)
	default:
		return nil, fmt.Errorf("unsupported value type %q; supported: %s", valueType, strings.Join(supportedTypes, ", "))
	}
}

func Decode(valueType string, data []byte) (any, error) {
	valueType = strings.ToLower(strings.TrimSpace(valueType))
	switch valueType {
	case "u8":
		if err := requireSize(data, 1); err != nil {
			return nil, err
		}
		return uint8(data[0]), nil
	case "i8":
		if err := requireSize(data, 1); err != nil {
			return nil, err
		}
		return int8(data[0]), nil
	case "u16":
		if err := requireSize(data, 2); err != nil {
			return nil, err
		}
		return binary.LittleEndian.Uint16(data), nil
	case "i16":
		if err := requireSize(data, 2); err != nil {
			return nil, err
		}
		return int16(binary.LittleEndian.Uint16(data)), nil
	case "u32":
		if err := requireSize(data, 4); err != nil {
			return nil, err
		}
		return binary.LittleEndian.Uint32(data), nil
	case "i32":
		if err := requireSize(data, 4); err != nil {
			return nil, err
		}
		return int32(binary.LittleEndian.Uint32(data)), nil
	case "u64":
		if err := requireSize(data, 8); err != nil {
			return nil, err
		}
		return binary.LittleEndian.Uint64(data), nil
	case "i64":
		if err := requireSize(data, 8); err != nil {
			return nil, err
		}
		return int64(binary.LittleEndian.Uint64(data)), nil
	case "f32":
		if err := requireSize(data, 4); err != nil {
			return nil, err
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(data)), nil
	case "f64":
		if err := requireSize(data, 8); err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(data)), nil
	case "utf8":
		return string(data), nil
	case "utf16le":
		if len(data)%2 != 0 {
			return nil, fmt.Errorf("UTF-16LE data length must be even, got %d", len(data))
		}
		codeUnits := make([]uint16, len(data)/2)
		for index := range codeUnits {
			codeUnits[index] = binary.LittleEndian.Uint16(data[index*2:])
		}
		return string(utf16.Decode(codeUnits)), nil
	case "hex":
		return strings.ToUpper(hex.EncodeToString(data)), nil
	default:
		return nil, fmt.Errorf("unsupported value type %q; supported: %s", valueType, strings.Join(supportedTypes, ", "))
	}
}

func ParseHex(value string) ([]byte, error) {
	normalized := strings.NewReplacer(" ", "", "\t", "", "\n", "", "_", "", "0x", "", "0X", "").Replace(value)
	if normalized == "" {
		return nil, fmt.Errorf("hex value is empty")
	}
	if len(normalized)%2 != 0 {
		return nil, fmt.Errorf("hex value must contain an even number of digits")
	}
	decoded, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid hex value: %w", err)
	}
	return decoded, nil
}

func ParsePattern(value string) ([]byte, []byte, error) {
	tokens := strings.Fields(strings.ReplaceAll(strings.TrimSpace(value), ",", " "))
	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("pattern is empty")
	}
	pattern := make([]byte, len(tokens))
	mask := make([]byte, len(tokens))
	for index, token := range tokens {
		switch token {
		case "?", "??", "**":
			mask[index] = '?'
		default:
			if len(token) != 2 {
				return nil, nil, fmt.Errorf("pattern token %q must be two hex digits or ??", token)
			}
			decoded, err := hex.DecodeString(token)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid pattern token %q: %w", token, err)
			}
			pattern[index] = decoded[0]
			mask[index] = 'x'
		}
	}
	return pattern, mask, nil
}

func ByteSize(valueType string) (uint32, bool) {
	switch strings.ToLower(valueType) {
	case "u8", "i8":
		return 1, true
	case "u16", "i16":
		return 2, true
	case "u32", "i32", "f32":
		return 4, true
	case "u64", "i64", "f64":
		return 8, true
	default:
		return 0, false
	}
}

func HexDump(data []byte, baseAddress uint64) string {
	var builder strings.Builder
	for offset := 0; offset < len(data); offset += 16 {
		end := min(offset+16, len(data))
		chunk := data[offset:end]
		fmt.Fprintf(&builder, "%016X  ", baseAddress+uint64(offset))
		for index := 0; index < 16; index++ {
			if index < len(chunk) {
				fmt.Fprintf(&builder, "%02X ", chunk[index])
			} else {
				builder.WriteString("   ")
			}
			if index == 7 {
				builder.WriteByte(' ')
			}
		}
		builder.WriteString(" |")
		for _, value := range chunk {
			if value >= 32 && value <= 126 {
				builder.WriteByte(value)
			} else {
				builder.WriteByte('.')
			}
		}
		builder.WriteString("|\n")
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

func encodeUint16(value uint16) []byte {
	encoded := make([]byte, 2)
	binary.LittleEndian.PutUint16(encoded, value)
	return encoded
}

func encodeUint32(value uint32) []byte {
	encoded := make([]byte, 4)
	binary.LittleEndian.PutUint32(encoded, value)
	return encoded
}

func encodeUint64(value uint64) []byte {
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, value)
	return encoded
}

func wrapParse(err error, valueType, value string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("invalid %s value %q: %w", valueType, value, err)
}

func requireSize(data []byte, size int) error {
	if len(data) < size {
		return fmt.Errorf("need at least %d bytes, got %d", size, len(data))
	}
	return nil
}
