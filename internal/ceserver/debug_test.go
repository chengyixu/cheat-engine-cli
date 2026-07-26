package ceserver

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestContinueModeForEvent(t *testing.T) {
	testCases := []struct {
		signal int32
		want   DebugContinueMode
	}{
		{signal: -2, want: DebugContinueIgnoreSignal},
		{signal: -1, want: DebugContinueIgnoreSignal},
		{signal: 5, want: DebugContinueIgnoreSignal},
		{signal: 19, want: DebugContinueIgnoreSignal},
		{signal: 11, want: DebugContinueDeliverSignal},
	}
	for _, testCase := range testCases {
		if got := continueModeForEvent(DebugContinueAuto, DebugEvent{Signal: testCase.signal}); got != testCase.want {
			t.Fatalf("signal %d: got %s, want %s", testCase.signal, got.String(), testCase.want.String())
		}
	}
	if got := continueModeForEvent(DebugContinueSingleStep, DebugEvent{Signal: 11}); got != DebugContinueSingleStep {
		t.Fatalf("explicit mode changed to %s", got.String())
	}
}

func TestValidateThreadContext(t *testing.T) {
	valid := make([]byte, 16)
	binary.LittleEndian.PutUint32(valid[:4], uint32(len(valid)))
	if err := validateThreadContext(valid); err != nil {
		t.Fatal(err)
	}
	invalid := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(invalid[:4], 8)
	if err := validateThreadContext(invalid); err == nil {
		t.Fatal("expected a size-header validation error")
	}
}

func TestTraceDebugEventsRejectsEventTimeoutAtNetworkDeadline(t *testing.T) {
	client := &Client{timeout: time.Second}
	if _, err := client.TraceDebugEvents(t.Context(), 42, 1, time.Second, DebugContinueAuto); err == nil {
		t.Fatal("expected event timeout validation error")
	}
}
