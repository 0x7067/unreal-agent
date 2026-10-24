package taskgraph

import (
	"fmt"
	"reflect"
	"testing"
)

func validationCheckpoint(t *testing.T, dependent, conflict bool, concurrency int) Graph {
	t.Helper()
	tasks := []Task{
		{ID: "a", Command: "true", Reads: []string{}, Writes: []string{"shared"}, Acceptance: "verify"},
		{ID: "b", Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "verify"},
	}
	if dependent {
		tasks[1].DependsOn = []string{"a"}
	}
	if conflict {
		tasks[1].Reads = []string{"shared"}
	}
	g, err := NewGraph("checkpoint", tasks, concurrency)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// Check both successful and rejected validation paths for accidental mutation.
func checkValidation(t *testing.T, g Graph, valid bool) {
	t.Helper()
	before := g.clone()
	err := g.Validate()
	if (err == nil) != valid {
		t.Errorf("Validate() = %v, want valid=%v; nodes=%+v", err, valid, g.Nodes)
	}
	if !reflect.DeepEqual(g, before) {
		t.Fatal("Validate mutated checkpoint")
	}
}

func TestCheckpointReservedStatusPairs(t *testing.T) {
	for _, a := range []string{Running, Canceling, Completed} {
		for _, b := range []string{Running, Canceling, Completed} {
			for _, stale := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stale=%v", a, b, stale), func(t *testing.T) {
					g := validationCheckpoint(t, false, true, 2)
					g.Nodes[0].Status, g.Nodes[1].Status = a, b
					g.Nodes[0].Stale = stale && a != Completed
					g.Nodes[1].Stale = stale && b != Completed
					checkValidation(t, g, false)
					// Read/read overlap is legal even while reservations are held.
					g.Nodes[0].Task.Reads = []string{"shared"}
					g.Nodes[0].Task.Writes = []string{}
					checkValidation(t, g, true)
				})
			}
		}
	}
}

func TestCheckpointActiveConcurrency(t *testing.T) {
	for _, a := range []string{Running, Canceling} {
		for _, b := range []string{Running, Canceling} {
			for _, stale := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stale=%v", a, b, stale), func(t *testing.T) {
					g := validationCheckpoint(t, false, false, 1)
					g.Nodes[0].Status, g.Nodes[1].Status = a, b
					g.Nodes[1].Stale = stale
					checkValidation(t, g, false)
					g.Concurrency = 2
					checkValidation(t, g, true)
					g.Concurrency = 1
					g.Nodes[1].Status, g.Nodes[1].Stale = Completed, false
					checkValidation(t, g, true) // Completed holds claims, not a worker slot.
				})
			}
		}
	}
}

func TestCheckpointPrerequisiteGating(t *testing.T) {
	for _, status := range []string{Running, Canceling, Completed, Accepted} {
		for _, prerequisite := range []string{Pending, Running, Canceling, Completed, Failed, Canceled, Accepted} {
			t.Run(status+"/prerequisite="+prerequisite, func(t *testing.T) {
				g := validationCheckpoint(t, true, false, 2)
				g.Nodes[0].Status, g.Nodes[0].Evidence = prerequisite, "verified"
				g.Nodes[1].Status, g.Nodes[1].Evidence = status, "verified"
				checkValidation(t, g, prerequisite == Accepted)
				if prerequisite == Running || prerequisite == Canceling {
					g.Nodes[0].Stale = true
					checkValidation(t, g, false)
				}
			})
		}
	}
}

func TestCheckpointStaleActiveAfterInvalidate(t *testing.T) {
	for _, status := range []string{Running, Canceling} {
		t.Run(status, func(t *testing.T) {
			g := validationCheckpoint(t, true, false, 1)
			if err := g.Start("a"); err != nil {
				t.Fatal(err)
			}
			if err := g.Complete("a", 1, "result", nil); err != nil {
				t.Fatal(err)
			}
			if err := g.Accept("a", 1, "verified"); err != nil {
				t.Fatal(err)
			}
			if err := g.Start("b"); err != nil {
				t.Fatal(err)
			}
			if status == Canceling {
				if err := g.Cancel("b"); err != nil {
					t.Fatal(err)
				}
			}
			if err := g.Invalidate("a"); err != nil {
				t.Fatal(err)
			}
			if !g.Nodes[1].Stale || g.Nodes[0].Status != Pending {
				t.Fatal("fixture did not invalidate prerequisite of active task")
			}
			checkValidation(t, g, true)
		})
	}
}

// Enumerate arbitrary restored states, not only states reachable through the
// reducer. The oracle intentionally uses no active/reserved/conflicts helpers.
func TestCheckpointBoundedTwoTaskOracle(t *testing.T) {
	statuses := []string{Pending, Running, Canceling, Completed, Accepted, Failed, Canceled}
	cases := 0
	for _, dependent := range []bool{false, true} {
		for _, conflict := range []bool{false, true} {
			for _, cap := range []int{1, 2} {
				for _, a := range statuses {
					for _, b := range statuses {
						for bits := 0; bits < 4; bits++ {
							g := validationCheckpoint(t, dependent, conflict, cap)
							g.Nodes[0].Status, g.Nodes[1].Status = a, b
							g.Nodes[0].Evidence, g.Nodes[1].Evidence = "verified", "verified"
							g.Nodes[0].Stale, g.Nodes[1].Stale = bits&1 != 0, bits&2 != 0
							valid, count, held := true, 0, 0
							for _, n := range g.Nodes {
								isActive := n.Status == Running || n.Status == Canceling
								if n.Stale && !isActive {
									valid = false
								}
								if isActive {
									count++
								}
								if isActive || n.Status == Completed {
									held++
								}
							}
							if count > cap || (conflict && held == 2) {
								valid = false
							}
							needsPrerequisite := b == Accepted || b == Completed || ((b == Running || b == Canceling) && !g.Nodes[1].Stale)
							if dependent && needsPrerequisite && (a != Accepted || g.Nodes[0].Stale) {
								valid = false
							}
							checkValidation(t, g, valid)
							cases++
						}
					}
				}
			}
		}
	}
	t.Logf("checked %d arbitrary two-task checkpoints", cases)
}
