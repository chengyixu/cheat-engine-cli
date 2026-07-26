package ceserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

type DebugContinueMode int32

const (
	DebugContinueAuto          DebugContinueMode = -1
	DebugContinueDeliverSignal DebugContinueMode = iota
	DebugContinueIgnoreSignal
	DebugContinueSingleStep
)

type BreakpointType int32

const (
	BreakpointExecute BreakpointType = iota
	BreakpointWrite
	BreakpointRead
	BreakpointAccess
)

type BreakpointCapabilities struct {
	Execute int `json:"execute"`
	Watch   int `json:"watch"`
	Shared  int `json:"shared"`
}

type DebugEvent struct {
	Signal       int32                   `json:"signal"`
	SignalName   string                  `json:"signal_name"`
	Kind         string                  `json:"kind"`
	ThreadID     uint64                  `json:"thread_id"`
	Address      uint64                  `json:"address,omitempty"`
	Capabilities *BreakpointCapabilities `json:"breakpoint_capabilities,omitempty"`
}

type DebugTrace struct {
	PID                      int32             `json:"pid"`
	Events                   []DebugEvent      `json:"events"`
	EventCount               int               `json:"event_count"`
	MaximumEvents            int               `json:"maximum_events"`
	EventTimeoutMilliseconds int32             `json:"event_timeout_ms"`
	TimedOut                 bool              `json:"timed_out"`
	ContinueMode             DebugContinueMode `json:"continue_mode_code"`
	ContinueModeName         string            `json:"continue_mode"`
}

type ThreadContext struct {
	StructSize uint32 `json:"struct_size"`
	TypeCode   uint32 `json:"type_code"`
	Bytes      []byte `json:"bytes"`
}

func (mode DebugContinueMode) String() string {
	switch mode {
	case DebugContinueAuto:
		return "auto"
	case DebugContinueDeliverSignal:
		return "deliver"
	case DebugContinueIgnoreSignal:
		return "ignore"
	case DebugContinueSingleStep:
		return "single-step"
	default:
		return "unknown"
	}
}

func (client *Client) SuspendThread(pid, tid int32) (int32, error) {
	return client.changeThreadSuspension(pid, tid, commandSuspendThread, "suspend thread")
}

func (client *Client) ResumeThread(pid, tid int32) (int32, error) {
	return client.changeThreadSuspension(pid, tid, commandResumeThread, "resume thread")
}

func (client *Client) changeThreadSuspension(pid, tid int32, command byte, operation string) (int32, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(command)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, tid)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return 0, err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return 0, client.protocolError(operation, err)
	}
	if result < 0 {
		return 0, &ProtocolError{Operation: operation, Message: fmt.Sprintf("server could not change TID %d; an active debug session is required", tid)}
	}
	return result, nil
}

func (client *Client) SetBreakpoint(pid, tid, debugRegister int32, address uint64, breakpointType BreakpointType, size int32) (bool, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return false, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 29))
	packet.WriteByte(commandSetBreakpoint)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, tid)
	_ = binary.Write(packet, binary.LittleEndian, debugRegister)
	_ = binary.Write(packet, binary.LittleEndian, address)
	_ = binary.Write(packet, binary.LittleEndian, int32(breakpointType))
	_ = binary.Write(packet, binary.LittleEndian, size)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return false, client.protocolError("set breakpoint", err)
	}
	return result != 0, nil
}

func (client *Client) RemoveBreakpoint(pid, tid, debugRegister int32, wasWatchpoint bool) (bool, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return false, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	packet := bytes.NewBuffer(make([]byte, 0, 17))
	packet.WriteByte(commandRemoveBreakpoint)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, tid)
	_ = binary.Write(packet, binary.LittleEndian, debugRegister)
	var watchpoint uint32
	if wasWatchpoint {
		watchpoint = 1
	}
	_ = binary.Write(packet, binary.LittleEndian, watchpoint)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return false, client.protocolError("remove breakpoint", err)
	}
	return result != 0, nil
}

func (client *Client) GetThreadContext(pid, tid int32) (ThreadContext, error) {
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return ThreadContext{}, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	return client.getThreadContext(handle, tid)
}

func (client *Client) getThreadContext(handle uint32, tid int32) (ThreadContext, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(commandGetThreadContext)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, uint32(tid))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return ThreadContext{}, err
	}
	var result uint32
	if err := client.readValue(&result); err != nil {
		return ThreadContext{}, client.protocolError("get thread context", err)
	}
	if result == 0 {
		return ThreadContext{}, &ProtocolError{Operation: "get thread context", Message: fmt.Sprintf("server could not read TID %d", tid)}
	}
	var size uint32
	if err := client.readValue(&size); err != nil {
		return ThreadContext{}, client.protocolError("get thread context size", err)
	}
	if size < 8 || size > maximumThreadContextSize {
		return ThreadContext{}, &ProtocolError{Operation: "get thread context", Message: fmt.Sprintf("invalid context size %d", size)}
	}
	data := make([]byte, size)
	if err := client.readFull(data); err != nil {
		return ThreadContext{}, client.protocolError("get thread context data", err)
	}
	embeddedSize := binary.LittleEndian.Uint32(data[:4])
	if embeddedSize != size {
		return ThreadContext{}, &ProtocolError{Operation: "get thread context", Message: fmt.Sprintf("context header declares %d bytes, response contains %d", embeddedSize, size)}
	}
	return ThreadContext{StructSize: size, TypeCode: binary.LittleEndian.Uint32(data[4:8]), Bytes: data}, nil
}

func (client *Client) SetThreadContext(pid, tid int32, data []byte) (bool, error) {
	if err := validateThreadContext(data); err != nil {
		return false, err
	}
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return false, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	if err := client.startDebug(handle); err != nil {
		return false, err
	}
	packet := bytes.NewBuffer(make([]byte, 0, 13+len(data)))
	packet.WriteByte(commandSetThreadContext)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, uint32(tid))
	_ = binary.Write(packet, binary.LittleEndian, uint32(len(data)))
	packet.Write(data)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return false, err
	}
	var result uint32
	if err := client.readValue(&result); err != nil {
		return false, client.protocolError("set thread context", err)
	}
	return result != 0, nil
}

func validateThreadContext(data []byte) error {
	if len(data) < 8 || len(data) > maximumThreadContextSize {
		return fmt.Errorf("thread context must be between 8 and %d bytes", maximumThreadContextSize)
	}
	declaredSize := binary.LittleEndian.Uint32(data[:4])
	if uint64(declaredSize) != uint64(len(data)) {
		return fmt.Errorf("thread context header declares %d bytes, input contains %d", declaredSize, len(data))
	}
	return nil
}

func (client *Client) TraceDebugEvents(ctx context.Context, pid int32, maximumEvents int, eventTimeout time.Duration, continueMode DebugContinueMode) (DebugTrace, error) {
	if maximumEvents < 1 || maximumEvents > 10_000 {
		return DebugTrace{}, fmt.Errorf("maximum debug events must be between 1 and 10000")
	}
	if eventTimeout < time.Millisecond || eventTimeout > time.Duration(^uint32(0)>>1)*time.Millisecond {
		return DebugTrace{}, fmt.Errorf("debug event timeout must be between 1 millisecond and the signed 32-bit millisecond limit")
	}
	if eventTimeout >= client.timeout {
		return DebugTrace{}, fmt.Errorf("debug event timeout %s must be shorter than network timeout %s", eventTimeout, client.timeout)
	}
	if continueMode < DebugContinueAuto || continueMode > DebugContinueSingleStep {
		return DebugTrace{}, fmt.Errorf("invalid debug continue mode %d", continueMode)
	}
	handle, err := client.OpenProcess(pid)
	if err != nil {
		return DebugTrace{}, err
	}
	defer func() { _ = client.CloseHandle(handle) }()
	if err := client.startDebug(handle); err != nil {
		return DebugTrace{}, err
	}
	trace := DebugTrace{
		PID: pid, Events: make([]DebugEvent, 0, maximumEvents), MaximumEvents: maximumEvents,
		EventTimeoutMilliseconds: int32(eventTimeout / time.Millisecond), ContinueMode: continueMode, ContinueModeName: continueMode.String(),
	}
	for len(trace.Events) < maximumEvents {
		if err := ctx.Err(); err != nil {
			return DebugTrace{}, err
		}
		event, available, err := client.waitForDebugEvent(handle, trace.EventTimeoutMilliseconds)
		if err != nil {
			return DebugTrace{}, err
		}
		if !available {
			trace.TimedOut = true
			break
		}
		trace.Events = append(trace.Events, event)
		effectiveContinueMode := continueModeForEvent(continueMode, event)
		if err := client.continueFromDebugEvent(handle, event.ThreadID, effectiveContinueMode); err != nil {
			return DebugTrace{}, err
		}
	}
	trace.EventCount = len(trace.Events)
	return trace, nil
}

func continueModeForEvent(requested DebugContinueMode, event DebugEvent) DebugContinueMode {
	if requested != DebugContinueAuto {
		return requested
	}
	switch event.Signal {
	case -2, -1, 5, 19:
		return DebugContinueIgnoreSignal
	default:
		return DebugContinueDeliverSignal
	}
}

func (client *Client) startDebug(handle uint32) error {
	if client.debugActive {
		return nil
	}
	packet := bytes.NewBuffer(make([]byte, 0, 5))
	packet.WriteByte(commandStartDebug)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return err
	}
	client.debugActive = true
	var result int32
	if err := client.readValue(&result); err != nil {
		return client.protocolError("start debug session", err)
	}
	if result == 0 {
		client.debugActive = false
		return &ProtocolError{Operation: "start debug session", Message: "server could not attach to the target process"}
	}
	return nil
}

func (client *Client) waitForDebugEvent(handle uint32, timeoutMilliseconds int32) (DebugEvent, bool, error) {
	packet := bytes.NewBuffer(make([]byte, 0, 9))
	packet.WriteByte(commandWaitForDebugEvent)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, timeoutMilliseconds)
	if err := client.writePacket(packet.Bytes()); err != nil {
		return DebugEvent{}, false, err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return DebugEvent{}, false, client.protocolError("wait for debug event", err)
	}
	if result == 0 {
		return DebugEvent{}, false, nil
	}
	data := make([]byte, 20)
	if err := client.readFull(data); err != nil {
		return DebugEvent{}, false, client.protocolError("read debug event", err)
	}
	event := DebugEvent{Signal: int32(binary.LittleEndian.Uint32(data[:4])), ThreadID: binary.LittleEndian.Uint64(data[4:12])}
	event.SignalName, event.Kind = debugSignalName(event.Signal)
	switch event.Signal {
	case -2:
		event.Capabilities = &BreakpointCapabilities{Execute: int(data[12]), Watch: int(data[13]), Shared: int(data[14])}
	case 5:
		event.Address = binary.LittleEndian.Uint64(data[12:20])
	}
	return event, true, nil
}

func (client *Client) continueFromDebugEvent(handle uint32, threadID uint64, mode DebugContinueMode) error {
	if threadID > uint64(^uint32(0)) {
		return &ProtocolError{Operation: "continue debug event", Message: fmt.Sprintf("thread ID %d exceeds protocol width", threadID)}
	}
	packet := bytes.NewBuffer(make([]byte, 0, 13))
	packet.WriteByte(commandContinueFromDebugEvent)
	_ = binary.Write(packet, binary.LittleEndian, handle)
	_ = binary.Write(packet, binary.LittleEndian, uint32(threadID))
	_ = binary.Write(packet, binary.LittleEndian, int32(mode))
	if err := client.writePacket(packet.Bytes()); err != nil {
		return err
	}
	var result int32
	if err := client.readValue(&result); err != nil {
		return client.protocolError("continue debug event", err)
	}
	if result == 0 {
		return &ProtocolError{Operation: "continue debug event", Message: fmt.Sprintf("server could not continue TID %d", threadID)}
	}
	return nil
}

func debugSignalName(signal int32) (string, string) {
	switch signal {
	case -2:
		return "create-process", "create-process"
	case -1:
		return "create-thread", "create-thread"
	case 5:
		return "SIGTRAP", "trap"
	case 19:
		return "SIGSTOP", "stop"
	default:
		return fmt.Sprintf("signal-%d", signal), "signal"
	}
}
