package klf200

import (
	"context"
	"fmt"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// This file ports pyvlx's api/command_send.py and api/status_request.py to the
// Client (api/get_state.py is ported as Client.GetState in auth_system.go). Each
// function builds the protocol request
// frame, runs the request/response exchange via Client.APICall with a
// FrameHandler that correlates by session id and awaits the terminal frame, and
// exposes the results.

// CommandSend sends a positioning command to a single node and, per pyvlx
// CommandSend, optionally waits for the movement to finish. It ports
// api/command_send.py.
//
// parameter is the main parameter (target position sentinel or percentage). The
// functionalParameters map carries optional functional parameters keyed 1..16
// (fp1..fp16); pyvlx uses fp3 for blind orientation. The FPI indicator bytes are
// derived from which functional parameters are present, exactly as
// FrameCommandSendRequest.__init__ computes fpi1/fpi2.
//
// If waitForCompletion is false the call returns once the command is accepted
// (GW_COMMAND_SEND_CFM); if true it additionally waits for
// GW_SESSION_FINISHED_NTF. Ported from CommandSend.handle_frame / do_api_call.
func (c *Client) CommandSend(
	ctx context.Context,
	nodeID uint8,
	parameter protocol.Parameter,
	functionalParameters map[int]protocol.Parameter,
	waitForCompletion bool,
) error {
	sid := c.Sessions().NewSessionID()

	req := &protocol.FrameCommandSendRequest{
		SessionID:       sid,
		Originator:      protocol.OriginatorUser,
		Priority:        protocol.PriorityUserLevel2,
		ActiveParameter: 0,
		Parameter:       parameter,
	}
	// Derive FPI1/FPI2 and populate the functional-parameter slots, mirroring
	// FrameCommandSendRequest.__init__: for fp1..fp16, set bit (8-i) in fpi1 for
	// i<9 and bit (16-i) in fpi2 for i>=9.
	for i := 1; i <= 16; i++ {
		fp, ok := functionalParameters[i]
		if !ok {
			continue
		}
		req.FunctionalParameter[i-1] = fp.Raw
		if i < 9 {
			req.FPI1 += 1 << uint(8-i)
		} else {
			req.FPI2 += 1 << uint(16-i)
		}
	}
	req.NodeIDs = []uint8{nodeID}

	success := false
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {
		case *protocol.FrameCommandSendConfirmation:
			if f.SessionID != sid {
				return false
			}
			if f.Status == protocol.CommandSendConfirmationStatusAccepted {
				success = true
			}
			// If not waiting for completion, the confirmation is terminal.
			return !waitForCompletion
		case *protocol.FrameCommandRemainingTimeNotification:
			// Ignored (pyvlx ignores these), keep waiting.
			return false
		case *protocol.FrameCommandRunStatusNotification:
			// Ignored (pyvlx ignores these), keep waiting.
			return false
		case *protocol.FrameSessionFinishedNotification:
			if f.SessionID != sid {
				return false
			}
			return true
		default:
			return false
		}
	})
	if err != nil {
		return fmt.Errorf("klf200: command send: %w", err)
	}
	if !success {
		return fmt.Errorf("klf200: unable to send command")
	}
	return nil
}

// StatusResult holds the fields of a GW_STATUS_REQUEST_NTF relevant to callers.
// It ports the notification_frame that StatusRequest exposes on success.
type StatusResult struct {
	NodeID          uint8
	TargetPosition  protocol.Parameter
	CurrentPosition protocol.Parameter
	Notification    *protocol.FrameStatusRequestNotification
}

// StatusRequest requests the current status of a single node and returns the
// resulting notification. It ports api/status_request.py: it awaits the
// GW_STATUS_REQUEST_NTF matching the session id (the confirmation is not
// terminal, matching StatusRequest.handle_frame).
func (c *Client) StatusRequest(ctx context.Context, nodeID uint8) (*StatusResult, error) {
	sid := c.Sessions().NewSessionID()

	req := &protocol.FrameStatusRequestRequest{
		SessionID:  sid,
		NodeIDs:    []uint8{nodeID},
		StatusType: protocol.StatusTypeRequestCurrentPosition,
		FPI1:       254, // Request FP1..FP7 (== pyvlx default)
		FPI2:       0,
	}

	var result *StatusResult
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {
		case *protocol.FrameStatusRequestConfirmation:
			// Still waiting for the notification (pyvlx returns False here).
			return false
		case *protocol.FrameStatusRequestNotification:
			if f.SessionID != sid {
				return false
			}
			result = &StatusResult{
				NodeID:          f.NodeID,
				TargetPosition:  f.TargetPosition,
				CurrentPosition: f.CurrentPosition,
				Notification:    f,
			}
			return true
		default:
			return false
		}
	})
	if err != nil {
		return nil, fmt.Errorf("klf200: status request: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("klf200: unable to send command")
	}
	return result, nil
}

// GetState is declared in auth_system.go (returns GatewayState).
