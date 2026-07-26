package ceserver

import (
	"encoding/binary"
	"testing"
)

func TestParseSymbols(t *testing.T) {
	data := make([]byte, 17+4)
	binary.LittleEndian.PutUint64(data[0:8], 0x1234)
	binary.LittleEndian.PutUint32(data[8:12], 16)
	binary.LittleEndian.PutUint32(data[12:16], 2)
	data[16] = 4
	copy(data[17:], "main")
	symbols, err := parseSymbols(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].Name != "main" || symbols[0].Address != 0x1234 {
		t.Fatalf("symbols = %#v", symbols)
	}
}

func TestParseSymbolsRejectsTruncation(t *testing.T) {
	if _, err := parseSymbols([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected truncation error")
	}
}
