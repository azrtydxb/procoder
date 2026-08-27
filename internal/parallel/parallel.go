// Package parallel bounds how many external processes procoder runs at
// once — across the whole binary, not per fan-out.
//
// It exists because three independent fan-outs landed separately and each
// was sized NumCPU on its own: the formatter pass over every file, the
// secret scan over every file, and the gate's legs running together. Each
// was correct alone and measured faster alone. Stacked, they multiplied:
// a whole-tree run put twenty to thirty external processes on ten cores,
// went SLOWER than any single change had made it, and starved the mermaid
// checker until it hit its ninety-second timeout and reported two
// diagrams as NOT checked. A gate that blocks because it gave itself no
// CPU is worse than a slow one.
//
// One budget, taken once per unit of work, is what makes the fan-outs
// composable: a caller need not know what else is running.
package parallel

import "runtime"

// budget is the number of units of work that may be in flight. Sized to
// the machine, at least one — a zero-capacity channel would deadlock, and
// runtime.NumCPU has returned 0 on constrained containers.
var budget = make(chan struct{}, maxInt(1, runtime.NumCPU()))

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Do runs f for every index in [0,n), never more than the budget at once,
// and returns when all of them have finished.
//
// Results are the caller's to place: every fan-out in this repository
// writes into its own slot by index, because findings are printed in the
// order they were asked for and not the order they arrived.
func Do(n int, f func(i int)) {
	if n <= 0 {
		return
	}
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			budget <- struct{}{}
			defer func() {
				<-budget
				done <- struct{}{}
			}()
			f(i)
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}
}
