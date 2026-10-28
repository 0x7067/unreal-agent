package reminder

import "time"

// FakeClock is a manually controlled clock for deterministic hosts and tests.
// Like the reducer it is single-owner; callers synchronize any concurrent use.
// Set deliberately permits backward movement to test wall-clock correction.
type FakeClock struct{ Current time.Time }

func (c *FakeClock) Now() time.Time          { return c.Current }
func (c *FakeClock) Set(now time.Time)       { c.Current = now }
func (c *FakeClock) Advance(d time.Duration) { c.Current = c.Current.Add(d) }
