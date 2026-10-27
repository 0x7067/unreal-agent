// Package executionbudget provides live wall budgets and shared, nonblocking
// operation admission accounting. It is not a sandbox or an OS process limit.
package executionbudget

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Policy measures elapsed wall time, not output, CPU usage, or evidence of a
// stall. Zero disables a threshold. Timeout requests cancellation; it does not
// establish that execution has ended.
type Policy struct {
	WarnAfter time.Duration `json:",omitzero"`
	Timeout   time.Duration `json:",omitzero"`
}

func (p Policy) Validate() error {
	if p.WarnAfter < 0 || p.Timeout < 0 {
		return errors.New("wall budget thresholds must not be negative")
	}
	if p.Timeout > 0 && p.WarnAfter >= p.Timeout {
		return errors.New("warning threshold must precede timeout")
	}
	return nil
}

// Constrain combines an operation request and manager policy. A request may
// tighten but cannot disable or extend a configured manager threshold.
func Constrain(base, request Policy) Policy {
	minPositive := func(a, b time.Duration) time.Duration {
		if a == 0 {
			return b
		}
		if b == 0 {
			return a
		}
		return min(a, b)
	}
	p := Policy{WarnAfter: minPositive(base.WarnAfter, request.WarnAfter), Timeout: minPositive(base.Timeout, request.Timeout)}
	if p.Timeout > 0 && p.WarnAfter >= p.Timeout {
		p.WarnAfter = p.Timeout / 2
	}
	return p
}

type Limits struct {
	MaxOperations int
	// MaxManagementOperations reserves a separate bounded admission lane for
	// typed inspect/stop controls. Nonpositive selects the default of eight.
	MaxManagementOperations int
}
type Snapshot struct {
	// Scope is explicitly process-local: remote worker internals are not counted.
	Scope           string
	Limit           int
	WorkOwned       int
	ManagementLimit int
	ManagementOwned int
	Owned           int
	Peak            int
	Rejected        uint64
}

var ErrCapacity = errors.New("shared operation capacity exhausted")

type Account struct {
	mu              sync.Mutex
	limits          Limits
	owned, peak     int
	managementOwned int
	rejected        uint64
}

func NewAccount(limits Limits) *Account {
	if limits.MaxManagementOperations <= 0 {
		limits.MaxManagementOperations = 8
	}
	return &Account{limits: limits}
}

// Acquire reserves one operation envelope. Descendant managers must reuse this
// account to share its bound. A lease must not be released merely on timeout,
// reported result, or cancellation acknowledgment; release follows proven drain.
// Nonblocking rejection avoids deadlock when parents retain slots for children.
func (a *Account) Acquire() (*Lease, error) { return a.acquire(false) }

// AcquireManagement reserves the bounded lane used by typed inspection and
// cancellation operations. Callers must not route arbitrary executable work
// here: the operation manager maintains an explicit versioned control set.
func (a *Account) AcquireManagement() (*Lease, error) { return a.acquire(true) }
func (a *Account) acquire(management bool) (*Lease, error) {
	if a == nil {
		return nil, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	owned, limit := a.owned-a.managementOwned, a.limits.MaxOperations
	if management {
		owned, limit = a.managementOwned, a.limits.MaxManagementOperations
	}
	if limit > 0 && owned >= limit {
		a.rejected++
		return nil, fmt.Errorf("%w (management=%t, limit %d)", ErrCapacity, management, limit)
	}
	a.owned++
	if management {
		a.managementOwned++
	}
	a.peak = max(a.peak, a.owned)
	return &Lease{account: a, management: management}, nil
}
func (a *Account) Snapshot() Snapshot {
	if a == nil {
		return Snapshot{Scope: "in_process_operation_envelopes"}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return Snapshot{Scope: "in_process_operation_envelopes", Limit: a.limits.MaxOperations, WorkOwned: a.owned - a.managementOwned, ManagementLimit: a.limits.MaxManagementOperations, ManagementOwned: a.managementOwned, Owned: a.owned, Peak: a.peak, Rejected: a.rejected}
}

type Lease struct {
	once       sync.Once
	account    *Account
	management bool
}

func (l *Lease) Release() {
	if l != nil {
		l.once.Do(func() {
			l.account.mu.Lock()
			l.account.owned--
			if l.management {
				l.account.managementOwned--
			}
			l.account.mu.Unlock()
		})
	}
}

type accountKey struct{}

func ContextWithAccount(ctx context.Context, account *Account) context.Context {
	return context.WithValue(ctx, accountKey{}, account)
}
func AccountFromContext(ctx context.Context) *Account {
	a, _ := ctx.Value(accountKey{}).(*Account)
	return a
}

// Durations have an explicit numeric nanosecond representation. Go's JSON v2
// deliberately has no default time.Duration encoding; do not rely on options
// supplied by a caller or change this persisted representation implicitly.
func (p Policy) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		WarnAfter int64 `json:",omitzero"`
		Timeout   int64 `json:",omitzero"`
	}{int64(p.WarnAfter), int64(p.Timeout)})
}
func (p *Policy) UnmarshalJSON(data []byte) error {
	var wire struct {
		WarnAfter int64
		Timeout   int64
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	value := Policy{WarnAfter: time.Duration(wire.WarnAfter), Timeout: time.Duration(wire.Timeout)}
	if err := value.Validate(); err != nil {
		return err
	}
	*p = value
	return nil
}
