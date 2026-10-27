package executionbudget

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSharedAccountConcurrentBoundAndIdempotentDrain(t *testing.T) {
	a := NewAccount(Limits{MaxOperations: 3})
	ctx := ContextWithAccount(context.Background(), a)
	if AccountFromContext(ctx) != a {
		t.Fatal("account identity lost")
	}
	leases := make(chan *Lease, 100)
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			l, err := AccountFromContext(ctx).Acquire()
			if err == nil {
				leases <- l
			} else if !errors.Is(err, ErrCapacity) {
				t.Errorf("acquire: %v", err)
			}
		})
	}
	wg.Wait()
	close(leases)
	s := a.Snapshot()
	if s.Owned != 3 || s.Peak != 3 || s.Rejected != 97 {
		t.Fatalf("snapshot: %+v", s)
	}
	for l := range leases {
		l.Release()
		l.Release()
	}
	if a.Snapshot().Owned != 0 {
		t.Fatal("drained leases still counted")
	}
}
func TestConstrainPolicy(t *testing.T) {
	base := Policy{WarnAfter: time.Minute, Timeout: 2 * time.Minute}
	for _, request := range []Policy{{}, {WarnAfter: time.Hour, Timeout: 2 * time.Hour}, {Timeout: 10 * time.Second}} {
		got := Constrain(base, request)
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		if got.Timeout > base.Timeout || got.WarnAfter > base.WarnAfter {
			t.Fatalf("request expanded manager bound: %+v", got)
		}
	}
	for _, p := range []Policy{{WarnAfter: -1}, {Timeout: -1}, {WarnAfter: time.Second, Timeout: time.Second}} {
		if p.Validate() == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
}

func TestManagementLaneBoundedIndependentAndRetainedUntilRelease(t *testing.T) {
	a := NewAccount(Limits{MaxOperations: 1, MaxManagementOperations: 2})
	work, err := a.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.AcquireManagement()
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.AcquireManagement()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AcquireManagement(); !errors.Is(err, ErrCapacity) {
		t.Fatal("management lane unbounded")
	}
	if _, err := a.Acquire(); !errors.Is(err, ErrCapacity) {
		t.Fatal("management lane freed work capacity")
	}
	s := a.Snapshot()
	if s.Owned != 3 || s.WorkOwned != 1 || s.ManagementOwned != 2 || s.ManagementLimit != 2 {
		t.Fatalf("account: %+v", s)
	}
	first.Release()
	first.Release()
	replacement, err := a.AcquireManagement()
	if err != nil {
		t.Fatal(err)
	}
	if a.Snapshot().WorkOwned != 1 {
		t.Fatal("control drain released child work")
	}
	second.Release()
	replacement.Release()
	work.Release()
	if a.Snapshot().Owned != 0 {
		t.Fatal("leaked leases")
	}
	if NewAccount(Limits{}).Snapshot().ManagementLimit != 8 {
		t.Fatal("wrong management default")
	}
}
