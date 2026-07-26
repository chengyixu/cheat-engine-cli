package memory

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeInt32(t *testing.T) {
	encoded, err := Encode("i32", "-42")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode("i32", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != int32(-42) {
		t.Fatalf("decoded = %v, want -42", decoded)
	}
}

func TestParsePattern(t *testing.T) {
	pattern, mask, err := ParsePattern("48 8B ?? FF")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pattern, []byte{0x48, 0x8B, 0x00, 0xFF}) {
		t.Fatalf("pattern = %v", pattern)
	}
	if string(mask) != "xx?x" {
		t.Fatalf("mask = %q", mask)
	}
}

func TestParseAddress(t *testing.T) {
	address, err := ParseAddress("0x7fff_1234")
	if err != nil {
		t.Fatal(err)
	}
	if address != 0x7fff1234 {
		t.Fatalf("address = %#x", address)
	}
}
