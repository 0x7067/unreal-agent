package workerruntime

import (
	"context"
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"reflect"
	"testing"
)

type m27InitializerSpy struct {
	sessionstore.Store
	err   error
	calls int
	id    session.ID
	input inbox.ID
	op    operation.Operation
}

func (s *m27InitializerSpy) InitializeControlOperation(_ context.Context, id session.ID, input inbox.ID, op operation.Operation) error {
	s.calls++
	s.id = id
	s.input = input
	s.op = op
	return s.err
}
func TestM27StoreDecoratorPreservesStructuralControlCommit(t *testing.T) {
	for _, failure := range []bool{false, true} {
		sentinel := errors.New("injected atomic initialization failure")
		spy := &m27InitializerSpy{}
		if failure {
			spy.err = sentinel
		}
		var initializer sessionstore.ControlOperationInitializer = committedCheckpointStore{Store: spy, owners: &ownedExecution{}}
		op := m27Run(t)
		err := initializer.InitializeControlOperation(t.Context(), "parent", "control-input", op)
		if !errors.Is(err, spy.err) || spy.calls != 1 || spy.id != "parent" || spy.input != "control-input" || !reflect.DeepEqual(spy.op, op) {
			t.Fatalf("optional atomic control initialization not delegated: %v %+v", err, spy)
		}
	}
	if err := (committedCheckpointStore{Store: &m27CheckpointSpy{}, owners: &ownedExecution{}}).InitializeControlOperation(t.Context(), "parent", "input", operation.Operation{}); err == nil {
		t.Fatal("unsupported optional initializer falsely succeeded")
	}
}
