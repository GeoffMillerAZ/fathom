package commandcenter

import (
	"context"
	"fmt"
	"time"

	workflowengine "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/orchestration/workflow-engine"
	reviewqueue "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/review-queue"
	telem "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
)

// CommandCenter is the cross-workflow command surface. It manages workflow
// lifecycle, routes review items through the review-queue, and emits
// telemetry events to the injected sink for every state transition.
//
// CommandCenter is NOT safe for concurrent use. Callers that share a
// CommandCenter across goroutines must serialise access externally.
//
// The zero value is not usable — construct via New.
type CommandCenter struct {
	sink    sink.Sink
	records map[string]*WorkflowRecord
	queue   reviewqueue.Queue
	idSeq   int
	now     func() time.Time
}

// New constructs a CommandCenter that emits telemetry to s.
//
// s must not be nil; use sink.NewMultiSink() with no arguments for a
// no-op sink in tests that do not need to observe events.
func New(s sink.Sink) (*CommandCenter, error) {
	if s == nil {
		return nil, fmt.Errorf("commandcenter: sink must not be nil")
	}
	return &CommandCenter{
		sink:    s,
		records: make(map[string]*WorkflowRecord),
		now:     time.Now,
	}, nil
}

// Submit registers a workflow with the CommandCenter and creates its first
// WorkflowInstance at the definition's InitialState. A "workflow_submitted"
// telemetry event is emitted.
//
// The returned InstanceID is used for subsequent Advance and Pending calls.
func (cc *CommandCenter) Submit(spec WorkflowSpec) (string, error) {
	engine, err := workflowengine.NewEngine(spec.Definition)
	if err != nil {
		return "", fmt.Errorf("commandcenter: invalid workflow definition: %w", err)
	}
	// Inject a deterministic time source if the command center has one.
	engine.Now = cc.now

	cc.idSeq++
	instanceID := fmt.Sprintf("wf-%04d", cc.idSeq)

	inst := engine.NewInstance(instanceID)
	if spec.Context != nil {
		for k, v := range spec.Context {
			inst.Context[k] = v
		}
	}

	record := &WorkflowRecord{
		InstanceID: instanceID,
		Instance:   inst,
		Engine:     engine,
	}
	cc.records[instanceID] = record

	cc.emit(telem.Event{
		ID:     cc.eventID("submit", instanceID),
		Kind:   telem.KindInfo,
		Source: "commandcenter",
		Attributes: map[string]string{
			"event_type":    "workflow_submitted",
			"instance_id":   instanceID,
			"workflow_id":   spec.Definition.ID,
			"initial_state": spec.Definition.InitialState,
		},
	})

	return instanceID, nil
}

// Advance fires event on the workflow instance identified by instanceID. On
// success the instance transitions to the next state and a
// "workflow_advanced" telemetry event is emitted.
//
// When the transition moves the instance to a terminal state a review item is
// automatically enqueued with PriorityCritical so operators are notified.
func (cc *CommandCenter) Advance(instanceID, event string) error {
	rec, ok := cc.records[instanceID]
	if !ok {
		return fmt.Errorf("commandcenter: unknown instance %q", instanceID)
	}

	nextInst, err := rec.Engine.Advance(context.Background(), rec.Instance, event)
	if err != nil {
		return fmt.Errorf("commandcenter: advancing instance %q on event %q: %w", instanceID, event, err)
	}
	rec.Instance = *nextInst

	// Emit telemetry for the transition.
	cc.emit(telem.Event{
		ID:     cc.eventID("advance", instanceID),
		Kind:   telem.KindInfo,
		Source: "commandcenter",
		Attributes: map[string]string{
			"event_type":  "workflow_advanced",
			"instance_id": instanceID,
			"workflow_id": rec.Instance.WorkflowID,
			"event":       event,
			"state":       rec.Instance.CurrentState,
		},
	})

	// If the instance has reached a terminal state, enqueue a review item.
	for _, s := range rec.Engine.Definition.States {
		if s.ID == rec.Instance.CurrentState && s.Terminal {
			ri := reviewqueue.ReviewItem{
				ID:       fmt.Sprintf("rv-%s-%s", instanceID, rec.Instance.CurrentState),
				Priority: reviewqueue.PriorityCritical,
				Payload:  instanceID,
			}
			if err := cc.queue.Enqueue(ri); err != nil {
				// Enqueue errors are non-fatal but surfaced as telemetry.
				cc.emit(telem.Event{
					ID:     cc.eventID("enqueue-err", instanceID),
					Kind:   telem.KindError,
					Source: "commandcenter",
					Attributes: map[string]string{
						"event_type":  "review_enqueue_error",
						"instance_id": instanceID,
						"error":       err.Error(),
					},
				})
			}
			break
		}
	}

	return nil
}

// Pending returns all items currently in the review queue without removing
// them. Items are returned in priority order (highest first).
func (cc *CommandCenter) Pending() []PendingReview {
	n := cc.queue.Len()
	if n == 0 {
		return nil
	}

	// Drain and re-enqueue to get a snapshot without mutating the queue.
	drained := make([]reviewqueue.ReviewItem, 0, n)
	for range n {
		item, err := cc.queue.Dequeue()
		if err != nil {
			break
		}
		drained = append(drained, item)
	}
	for _, item := range drained {
		_ = cc.queue.Enqueue(item)
	}

	out := make([]PendingReview, len(drained))
	for i, item := range drained {
		instanceID, _ := item.Payload.(string)
		out[i] = PendingReview{Item: item, InstanceID: instanceID}
	}
	return out
}

// Instance returns the current WorkflowInstance for the given instanceID.
// Returns false if the instance is unknown.
func (cc *CommandCenter) Instance(instanceID string) (workflowengine.WorkflowInstance, bool) {
	rec, ok := cc.records[instanceID]
	if !ok {
		return workflowengine.WorkflowInstance{}, false
	}
	return rec.Instance, true
}

// ── private helpers ───────────────────────────────────────────────────────────

// emit normalises and forwards an event to the injected sink.
// Errors from the sink are silently discarded to prevent telemetry failures
// from breaking the workflow execution path.
func (cc *CommandCenter) emit(e telem.Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = cc.now()
	}
	e.Normalize()
	_ = cc.sink.Emit(e)
}

// eventID constructs a stable event ID from a verb and instance ID.
func (cc *CommandCenter) eventID(verb, instanceID string) string {
	return fmt.Sprintf("cc/%s/%s/%d", verb, instanceID, cc.now().UnixNano())
}
