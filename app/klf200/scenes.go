package klf200

// scenes.go ports scene.py, scenes.py, api/get_scene_list.py, and
// api/activate_scene.py.
//
// Scene  — a single scene with an ID and name; can be Run via GW_ACTIVATE_SCENE_REQ.
// Scenes — the thread-safe scene collection; loaded via LoadScenes (GW_GET_SCENE_LIST_REQ).
//
// api/wink_send.py is intentionally omitted — it deals with node winking, not
// scenes, and belongs with the node-command phase. discovery.py is also omitted
// as it performs LAN gateway UDP discovery and is not self-contained here.

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// ============================================================
// Scene
// ============================================================

// Scene represents a single KLF200 scene. It is the Go port of scene.py Scene.
type Scene struct {
	client  *Client
	SceneID uint8
	name    string
}

// Name returns the scene's human-readable name.
func (s *Scene) Name() string { return s.name }

// String returns a readable representation. Ported from Scene.__str__.
func (s *Scene) String() string {
	return fmt.Sprintf("<Scene name=%q id=%d>", s.name, s.SceneID)
}

// Run activates this scene on the gateway.
//
// waitForCompletion controls whether Run returns as soon as the confirmation is
// received (false) or waits until the GW_SESSION_FINISHED_NTF arrives (true).
// Ported from scene.py Scene.run / api/activate_scene.py ActivateScene.
func (s *Scene) Run(ctx context.Context, waitForCompletion bool) error {
	sid := s.client.Sessions().NewSessionID()
	req := &protocol.FrameActivateSceneRequest{
		SceneID:    s.SceneID,
		SessionID:  sid,
		Originator: protocol.OriginatorUser,
		Priority:   protocol.PriorityUserLevel2,
		Velocity:   protocol.VelocityDefault,
	}

	accepted := false
	err := s.client.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {

		case *protocol.FrameActivateSceneConfirmation:
			if f.SessionID != sid {
				return false
			}
			if f.Status == protocol.ActivateSceneConfirmationStatusAccepted {
				accepted = true
			}
			// If not waiting for completion, we are done here.
			return !waitForCompletion

		case *protocol.FrameCommandRemainingTimeNotification:
			// Ignore — pyvlx does the same.
			return false

		case *protocol.FrameCommandRunStatusNotification:
			// Ignore — pyvlx comments: "don't really understand what this is good for".
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
		return fmt.Errorf("klf200: activate scene %d: %w", s.SceneID, err)
	}
	if !accepted {
		return fmt.Errorf("klf200: activate scene %d: request rejected by gateway", s.SceneID)
	}
	return nil
}

// ============================================================
// Scenes
// ============================================================

// Scenes holds the collection of scenes loaded from the KLF200 gateway. It is
// the Go port of scenes.py Scenes.
type Scenes struct {
	mu     sync.RWMutex
	client *Client
	scenes []*Scene
}

// NewScenes constructs an empty Scenes collection for the given client.
func NewScenes(client *Client) *Scenes {
	return &Scenes{client: client}
}

// Add inserts or replaces a scene by SceneID. Ported from Scenes.add.
func (ss *Scenes) Add(scene *Scene) {
	if scene == nil {
		return
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for i, s := range ss.scenes {
		if s.SceneID == scene.SceneID {
			ss.scenes[i] = scene
			return
		}
	}
	ss.scenes = append(ss.scenes, scene)
}

// Clear removes all scenes from the collection. Ported from Scenes.clear.
func (ss *Scenes) Clear() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.scenes = nil
}

// Len returns the number of scenes. Ported from Scenes.__len__.
func (ss *Scenes) Len() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.scenes)
}

// ByID returns the scene with the given ID, or (nil, false) if not found.
// Ported from Scenes.__getitem__(int).
func (ss *Scenes) ByID(id uint8) (*Scene, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for _, s := range ss.scenes {
		if s.SceneID == id {
			return s, true
		}
	}
	return nil, false
}

// ByName returns the scene with the given name, or (nil, false) if not found.
// Ported from Scenes.__getitem__(str).
func (ss *Scenes) ByName(name string) (*Scene, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for _, s := range ss.scenes {
		if s.name == name {
			return s, true
		}
	}
	return nil, false
}

// All returns a snapshot copy of all scenes. Safe for iteration outside the lock.
func (ss *Scenes) All() []*Scene {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	out := make([]*Scene, len(ss.scenes))
	copy(out, ss.scenes)
	return out
}

// Load fetches the scene list from the gateway and populates the collection.
// Ported from scenes.py Scenes.load / api/get_scene_list.py GetSceneList.
//
// Wire flow:
//  1. Send GW_GET_SCENE_LIST_REQ.
//  2. Receive GW_GET_SCENE_LIST_CFM → captures expected count; if count=0 done.
//  3. Receive one or more GW_GET_SCENE_LIST_NTF → accumulate scenes;
//     done when remaining_scenes == 0.
func (ss *Scenes) Load(ctx context.Context) error {
	req := &protocol.FrameGetSceneListRequest{}

	var (
		expectedCount int = -1 // -1 means CFM not yet received
		collected     []protocol.SceneEntry
	)

	err := ss.client.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {

		case *protocol.FrameGetSceneListConfirmation:
			expectedCount = int(f.CountScenes)
			if expectedCount == 0 {
				return true // nothing to wait for
			}
			// Keep waiting for NTF frame(s).
			return false

		case *protocol.FrameGetSceneListNotification:
			collected = append(collected, f.Scenes...)
			if f.RemainingScenes != 0 {
				return false // more NTFs coming
			}
			// Log mismatch (pyvlx logs a warning; we surface it as an error below).
			return true

		default:
			return false
		}
	})
	if err != nil {
		return fmt.Errorf("klf200: load scenes: %w", err)
	}
	if expectedCount >= 0 && len(collected) != expectedCount {
		// Non-fatal — pyvlx only logs a warning. We surface the discrepancy but
		// still populate with what we got.
		err = fmt.Errorf("klf200: load scenes: expected %d scenes, got %d", expectedCount, len(collected))
	}

	ss.Clear()
	for _, entry := range collected {
		ss.Add(&Scene{
			client:  ss.client,
			SceneID: entry.Number,
			name:    entry.Name,
		})
	}
	return errors.Join(err) // nil if no discrepancy
}
