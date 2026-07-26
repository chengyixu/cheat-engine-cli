package ceserver

import (
	"reflect"
	"testing"
)

func TestScanBufferWithWildcardAndAlignment(t *testing.T) {
	data := []byte{0x48, 0x8B, 0x01, 0xFF, 0x48, 0x8B, 0x02, 0xFF}
	pattern := []byte{0x48, 0x8B, 0x00, 0xFF}
	mask := []byte("xx?x")
	matches := scanBuffer(nil, data, pattern, mask, 0x1000, 0x1000, 4, 0x1000, 10)
	want := []uint64{0x1000, 0x1004}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("matches = %#v, want %#v", matches, want)
	}
}

func TestScanBufferSkipsPreviouslyScannedTail(t *testing.T) {
	data := []byte{1, 2, 3, 1, 2, 3}
	pattern := []byte{1, 2, 3}
	mask := []byte("xxx")
	matches := scanBuffer(nil, data, pattern, mask, 0x2000, 0x2000, 1, 0x2003, 10)
	want := []uint64{0x2003}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("matches = %#v, want %#v", matches, want)
	}
}
