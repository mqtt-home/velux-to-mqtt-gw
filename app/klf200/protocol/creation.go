package protocol

import "sync"

// frameRegistry maps command codes to frame constructors. Concrete frame types
// register themselves here from init(), mirroring pyvlx's create_frame table.
var (
	frameRegistryMu sync.RWMutex
	frameRegistry   = make(map[Command]func() Frame)
)

// RegisterFrame registers a constructor for a command code. It is safe to call
// from init(). Registering the same command twice panics, to catch duplicate
// command codes at startup.
func RegisterFrame(cmd Command, constructor func() Frame) {
	frameRegistryMu.Lock()
	defer frameRegistryMu.Unlock()
	if _, exists := frameRegistry[cmd]; exists {
		panic("protocol: duplicate frame registration for command " + cmd.String())
	}
	frameRegistry[cmd] = constructor
}

// CreateFrame returns a new, empty Frame for the given command, or nil if no
// frame type is registered for it. Mirrors frame_creation.create_frame.
func CreateFrame(cmd Command) Frame {
	frameRegistryMu.RLock()
	constructor, ok := frameRegistry[cmd]
	frameRegistryMu.RUnlock()
	if !ok {
		return nil
	}
	return constructor()
}

// FrameFromRaw parses a raw (de-SLIPed) frame: it validates framing, looks up
// the constructor by command, and returns the populated frame.
//
// Following frame_creation.frame_from_raw semantics, an unknown command is not
// an error: it returns (nil, nil) so callers can log and skip it. A malformed
// frame (bad length/CRC) or a payload that fails to unmarshal returns an error.
func FrameFromRaw(raw []byte) (Frame, error) {
	command, payload, err := ExtractFromFrame(raw)
	if err != nil {
		return nil, err
	}
	frame := CreateFrame(command)
	if frame == nil {
		// Command not implemented: not an error (matches pyvlx warning+return None).
		return nil, nil
	}
	if err := frame.UnmarshalPayload(payload); err != nil {
		return nil, err
	}
	return frame, nil
}
