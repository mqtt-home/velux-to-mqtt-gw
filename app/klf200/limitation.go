package klf200

import (
	"context"
	"fmt"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// This file ports pyvlx's api/get_limitation.py and api/set_limitation.py to the
// Client: reading a node's current position limitation and setting/clearing it.

// LimitationResult holds the outcome of a GetLimitation call. It ports the
// fields GetLimitation exposes on success (min/max raw values, originator, limit
// time), plus the percentage accessors via MinValuePercent / MaxValuePercent.
type LimitationResult struct {
	MinValueRaw uint16
	MaxValueRaw uint16
	Originator  protocol.Originator
	LimitTime   uint8
}

// MinValuePercent returns the minimum limitation as a percentage, mirroring
// GetLimitation.min_value (Position.to_percent of the raw value).
func (r LimitationResult) MinValuePercent() int {
	return rawToPercent(r.MinValueRaw)
}

// MaxValuePercent returns the maximum limitation as a percentage, mirroring
// GetLimitation.max_value (Position.to_percent of the raw value).
func (r LimitationResult) MaxValuePercent() int {
	return rawToPercent(r.MaxValueRaw)
}

// rawToPercent converts a raw 2-byte limitation value to a percentage using the
// Position percentage rule. Mirrors Position.to_percent (int(raw[0]/2 + 0.5)).
func rawToPercent(raw uint16) int {
	high := byte(raw >> 8)
	return int(float64(high)/2 + 0.5)
}

// GetLimitation reads the limitation of the given node for the given limitation
// type and returns the resulting values. It ports api/get_limitation.py: the
// GW_GET_LIMITATION_STATUS_CFM is not terminal (pyvlx waits for the NTF); the
// GW_LIMITATION_STATUS_NTF matching the session id completes the call.
func (c *Client) GetLimitation(ctx context.Context, nodeID uint8, limitationType protocol.LimitationType) (*LimitationResult, error) {
	sid := c.Sessions().NewSessionID()

	req := &protocol.FrameGetLimitationStatusRequest{
		SessionID:       sid,
		NodeIDs:         []uint8{nodeID},
		ParameterID:     0, // Main Parameter
		LimitationsType: limitationType,
	}

	var result *LimitationResult
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {
		case *protocol.FrameGetLimitationStatusConfirmation:
			// Wait for the notification frame (pyvlx returns False here).
			return false
		case *protocol.FrameLimitationStatusNotification:
			if f.SessionID != sid {
				return false
			}
			result = &LimitationResult{
				MinValueRaw: f.MinValue,
				MaxValueRaw: f.MaxValue,
				Originator:  f.LimitOriginator,
				LimitTime:   f.LimitTime,
			}
			return true
		default:
			return false
		}
	})
	if err != nil {
		return nil, fmt.Errorf("klf200: get limitation: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("klf200: unable to send command")
	}
	return result, nil
}

// SetLimitation sets a node's minimum and maximum position limitation. It ports
// api/set_limitation.py. Per its NOTE, both limits must always be supplied
// together or the gateway rejects the frame; limitationTime carries the duration
// (Unlimited by default, ClearAll to clear). The GW_SET_LIMITATION_CFM is
// terminal; a REJECTED status is reported as failure.
func (c *Client) SetLimitation(
	ctx context.Context,
	nodeID uint8,
	limitationValueMin protocol.Position,
	limitationValueMax protocol.Position,
	limitationTime protocol.LimitationTime,
) error {
	sid := c.Sessions().NewSessionID()

	req := &protocol.FrameSetLimitationRequest{
		SessionID:      sid,
		Originator:     protocol.OriginatorUser,
		Priority:       protocol.PriorityUserLevel2,
		NodeIDs:        []uint8{nodeID},
		ParameterID:    0, // Main Parameter
		LimitationTime: limitationTime,
	}
	req.LimitationValueMin = limitationValueMin.Raw
	req.LimitationValueMax = limitationValueMax.Raw

	success := false
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		cfm, ok := frame.(*protocol.FrameSetLimitationConfirmation)
		if !ok {
			return false
		}
		if cfm.Status != protocol.SetLimitationRequestStatusRejected {
			success = true
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("klf200: set limitation: %w", err)
	}
	if !success {
		return fmt.Errorf("klf200: unable to set limitations")
	}
	return nil
}
