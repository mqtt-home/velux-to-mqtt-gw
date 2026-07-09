package klf200

import (
	"context"
	"fmt"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// HouseStatusMonitorEnable sends GW_HOUSE_STATUS_MONITOR_ENABLE_REQ and waits
// for the confirmation. Ported from house_status_monitor.HouseStatusMonitorEnable
// and house_status_monitor_enable (api/house_status_monitor.py).
func HouseStatusMonitorEnable(ctx context.Context, client *Client) error {
	req := &protocol.FrameHouseStatusMonitorEnableRequest{}
	err := client.APICall(ctx, req, func(frame protocol.Frame) bool {
		_, ok := frame.(*protocol.FrameHouseStatusMonitorEnableConfirmation)
		return ok
	})
	if err != nil {
		return fmt.Errorf("klf200: house status monitor enable: %w", err)
	}
	return nil
}

// HouseStatusMonitorDisable sends GW_HOUSE_STATUS_MONITOR_DISABLE_REQ and
// waits for the confirmation. Ported from house_status_monitor.HouseStatusMonitorDisable
// and house_status_monitor_disable (api/house_status_monitor.py).
func HouseStatusMonitorDisable(ctx context.Context, client *Client) error {
	req := &protocol.FrameHouseStatusMonitorDisableRequest{}
	err := client.APICall(ctx, req, func(frame protocol.Frame) bool {
		_, ok := frame.(*protocol.FrameHouseStatusMonitorDisableConfirmation)
		return ok
	})
	if err != nil {
		return fmt.Errorf("klf200: house status monitor disable: %w", err)
	}
	return nil
}
