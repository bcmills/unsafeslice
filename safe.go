// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !unsafe race

package unsafeslice

import (
	"fmt"

	"github.com/bcmills/unsafeslice/internal/eventually"
)

// This file contains declarations for “less unsafe” mode,
// which makes a best-effort attempt to detect string mutations.

// maybeDetectMutations makes a best effort to detect mutations and lifetime
// errors on the slice b. It is most effective when run under the race detector.
func maybeDetectMutations(b []byte) {
	if len(b) == 0 {
		return
	}

	c := newMutationChecker(b)

	if raceEnabled {
		// Start a goroutine that reads from the slice and does not have a
		// happens-before relationship with any other event in the program.
		//
		// Even if the goroutine is scheduled and runs to completion immediately, if
		// anything ever mutates the slice the race detector should report it as a
		// read/write race. The erroneous writer should be easy to identify from the
		// race report.
		go c.recheck()
	}

	// We can't set a finalizer on the slice contents itself, since we don't know
	// how it was allocated (or even whether it is owned by the Go runtime).
	// Instead, we use a finalizer on the mutation checker itself to make a best
	// effort to re-check the hash at some arbitrary future point in time.
	//
	// This attempts to produce a longer delay than scheduling a goroutine
	// immediately, in order to catch more mutations, but only extends the
	// lifetimes of allocated strings to the next GC cycle rather than by an
	// arbitrary time interval.
	//
	// However, because the lifetime of checksum is not tied to the lifetime of
	// the backing data in any way, this approach could backfire and run the check
	// much too early — before a dangerous mutation has even occurred. It's better
	// than nothing, but not an adequate substitute for the race-enabled version
	// of this check.
	eventually.SetFinalizer(c, (*mutationChecker).recheck)
}

type mutationChecker struct {
	b        []byte
	checksum uint64
}

func newMutationChecker(b []byte) *mutationChecker {
	c := &mutationChecker{b: b}
	c.checksum = c.sum64()
	return c
}

func (c *mutationChecker) recheck() {
	if c.sum64() != c.checksum {
		panic(fmt.Sprintf("mutation detected in string at address 0x%012x", &c.b[0]))
	}
}

func (c *mutationChecker) sum64() uint64 {
	h := newHash()
	initHash(h)

	h.Write(c.b)
	sum := h.Sum64()

	disposeHash(h)
	return sum
}
