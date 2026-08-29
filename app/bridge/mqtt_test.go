package bridge

import (
	"sync"
	"testing"
	"time"
)

// newBoundedMQTT builds an *MQTT with the publish timeout active (unlike
// newFakeMQTT, which leaves it at zero for deterministic ordering assertions)
// and a publishFn the test controls.
func newBoundedMQTT(timeout time.Duration, publishFn func(topic string, message any, retained bool)) *MQTT {
	return &MQTT{
		retain:         true,
		publishTimeout: timeout,
		publishFn:      publishFn,
	}
}

// TestPublishUnblocksCallerWhenBrokerStalls is the regression test for the
// 2026-08-29 outage: mqtt-gateway's publish ends in paho's untimed token.Wait(),
// so a broker outage blocked the caller forever. The caller that mattered was
// the health-check goroutine — parked on an availability publish, it never ran
// the reconnect/reboot/power-cycle ladder, and the covers stayed dead until a
// human intervened. A stalled broker must cost seconds, not the recovery path.
func TestPublishUnblocksCallerWhenBrokerStalls(t *testing.T) {
	stuck := make(chan struct{})
	defer close(stuck) // let the orphaned publish goroutine exit with the test

	var started sync.WaitGroup
	started.Add(1)
	m := newBoundedMQTT(20*time.Millisecond, func(_ string, _ any, _ bool) {
		started.Done()
		<-stuck
	})

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		m.PublishRetained("velux/vlx-office/available", "offline")
	}()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("PublishRetained did not return while the broker was stalled")
	}

	// The underlying publish was really attempted, not skipped: it is still
	// running, and will land if the broker comes back.
	started.Wait()
}

// TestPublishWaitsForCompletion asserts the timeout does not make publishing
// fire-and-forget in the normal case. Callers that publish and then read back
// their own effect (discovery before state, off before on in a power cycle)
// depend on the publish having happened by the time the call returns.
func TestPublishWaitsForCompletion(t *testing.T) {
	var mu sync.Mutex
	var got []string

	m := newBoundedMQTT(time.Second, func(topic string, message any, _ bool) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, topic+"="+message.(string))
	})

	m.Publish("hs100/velux_reset/set", "OFF", false)
	m.Publish("hs100/velux_reset/set", "ON", false)

	mu.Lock()
	defer mu.Unlock()
	want := []string{"hs100/velux_reset/set=OFF", "hs100/velux_reset/set=ON"}
	if len(got) != len(want) {
		t.Fatalf("published %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("publish[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

// TestPublishRetainedHonoursRetainFlag guards the plumbing through the new
// choke point: PublishRetained must still carry the configured retain flag, and
// Publish must still carry the caller's explicit one.
func TestPublishRetainedHonoursRetainFlag(t *testing.T) {
	type call struct {
		topic    string
		retained bool
	}
	var calls []call
	m := newBoundedMQTT(time.Second, func(topic string, _ any, retained bool) {
		calls = append(calls, call{topic, retained})
	})

	m.PublishRetained("velux/vlx-bath/position", "100")
	m.Publish("hs100/velux_reset/set", "OFF", false)

	if len(calls) != 2 {
		t.Fatalf("got %d publishes, want 2", len(calls))
	}
	if !calls[0].retained {
		t.Errorf("PublishRetained sent retained=false, want true")
	}
	if calls[1].retained {
		t.Errorf("Publish(retained=false) sent retained=true")
	}
}

// TestPublishWithoutTimeoutPassesThrough documents the zero-timeout escape
// hatch the bridge fakes rely on: no goroutine, no watchdog, straight call.
func TestPublishWithoutTimeoutPassesThrough(t *testing.T) {
	called := 0
	m := newBoundedMQTT(0, func(_ string, _ any, _ bool) { called++ })

	m.PublishRetained("velux/vlx-office/state", "closed")

	if called != 1 {
		t.Fatalf("publishFn called %d times, want 1", called)
	}
}
