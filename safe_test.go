// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !unsafe race

package unsafeslice_test

import (
	"bytes"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/bcmills/unsafeslice"
	"github.com/bcmills/unsafeslice/internal/eventually"
)

const maxStringAllocs = 1

// TestStringMutations verifies that OfString and AsString detect immediate
// mutations in string values, which are supposed to be immutable and
// persistent.
func TestStringMutations(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skipf("js does not support os/exec")
	}

	if os.Getenv("UNSAFESLICE_TEST_STRING_MUTATIONS") != "" {
		// Block "eventually" finalizers from running until we have actually mutated
		// the slices. This guarantees that the finalizer will detect the mutation,
		// even if the scheduler and collector are as antagonistic as possible.
		unblock := eventually.Block()

		t.Run("AsString", func(t *testing.T) {
			b := []byte("Hello, world!")
			_ = unsafeslice.AsString(b)
			copy(b, "Kaboom")
		})

		t.Run("OfString", func(t *testing.T) {
			// Construct a string backed by mutable memory, but avoid
			// unsafeslice.AsString so that we don't accidentally trigger its mutation
			// check instead.
			// (This test is not interesting if the attempt at mutating the string
			// faults immediately or fails for reasons unrelated to OfString.)
			buf := []byte("Hello, world!")
			var s string
			hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
			hdr.Data = uintptr(unsafe.Pointer(&buf[0]))
			hdr.Len = len(buf)

			b := unsafeslice.OfString(s)
			copy(b, "Kaboom")
		})

		unblock()
		var waste []*uint64
		for {
			runtime.GC()
			waste = append(waste, new(uint64)) // Allocate garbage to attempt to force finalizers to run.
		}
		runtime.KeepAlive(waste)
	}

	runSubtestProcess := func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command(os.Args[0], "-test.run="+t.Name(), "-test.v")
		cmd.Env = append(os.Environ(), "UNSAFESLICE_TEST_STRING_MUTATIONS=1")
		out := new(bytes.Buffer)
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}

		err := cmd.Wait()
		t.Logf("%s:\n%s", strings.Join(cmd.Args, " "), out)
		if err == nil {
			t.Errorf("Test subprocess passed; want a crash due to detected mutations.")
		}
	}

	t.Run("AsString", runSubtestProcess)
	t.Run("OfString", runSubtestProcess)
}
