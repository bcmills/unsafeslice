// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unsafeslice_test

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
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

// asCPointer returns b as a C-style pointer and length
func asCPointer(b []byte) (*byte, int) {
	if len(b) == 0 {
		return nil, 0
	}
	return &b[0], len(b)
}

func ExampleSetAt() {
	original := []byte("Hello, world!")
	p, n := asCPointer(original)

	var alias []byte
	unsafeslice.SetAt(&alias, unsafe.Pointer(p), n)

	fmt.Printf("original: %s\n", original)
	fmt.Printf("alias: %s\n", alias)
	copy(alias, "Adios")
	fmt.Printf("original: %s\n", original)
	fmt.Printf("alias: %s\n", alias)

	// Output:
	// original: Hello, world!
	// alias: Hello, world!
	// original: Adios, world!
	// alias: Adios, world!
}

func ExampleConvertAt() {
	// For this example, we're going to do a transformation on some ASCII text.
	// That transformation is not endian-sensitive, so we can reinterpret the text
	// as a slice of uint32s to process it word-at-a-time instead of
	// byte-at-a-time.

	const input = "HELLO, WORLD!"

	// Allocate an aligned backing buffer.
	buf := make([]uint32, (len(input)+3)/4)

	// Reinterpret it as a byte slice so that we can copy in our text.
	var alias []byte
	unsafeslice.ConvertAt(&alias, buf)
	copy(alias, input)

	// Perform an endian-insensitive transformation word-by-word instead of
	// byte-by-byte.
	for i := range buf {
		buf[i] |= 0x20202020
	}

	// Read the result back out of the byte-slice view to interpret it as text.
	fmt.Printf("%s\n", alias[:len(input)])

	// Output:
	// hello, world!
}

func TestSetAtWithVeryLargeTypeDoesNotPanic(t *testing.T) {
	type big [1 << 20]byte
	var x big
	var s []big
	unsafeslice.SetAt(&s, unsafe.Pointer(&x), 1)
}

func TestConvertAtErrors(t *testing.T) {
	cases := []struct {
		desc     string
		dst, src interface{}
	}{
		{
			desc: "incompatible capacity",
			src:  []byte("foobar")[:4:6],
			dst:  new([]uint32),
		},
		{
			desc: "incompatible length",
			src:  []byte("foobar\x00\x00")[:6],
			dst:  new([]uint32),
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			defer func() {
				if msg := recover(); msg != nil {
					t.Logf("recovered: %v", msg)
				} else {
					t.Errorf("ConvertAt failed to panic as expected.")
				}
			}()

			unsafeslice.ConvertAt(tc.dst, tc.src)
		})
	}
}

func ExampleOfString() {
	s := "Hello, world!"

	// Temporarily view the string s as a slice,
	// so that we can compute its checksum with an arbitrary hash.Hash
	// implementation without needing to copy it.
	var h hash.Hash = fnv.New64a()
	b := unsafeslice.OfString(s)

	// This is safe because the contract for an io.Writer requires:
	// > Write must not modify the slice data, even temporarily.
	h.Write(b)

	fmt.Printf("%x\n", h.Sum(nil))

	// Output:
	// 38d1334144987bf4
}

func ExampleToString() {
	const input = "Hello, world!"
	h := fnv.New64a()
	io.WriteString(h, input)

	// Save the computed checksum as an immutable string,
	// without incurring any additional allocations or copying
	// (beyond the slice for the Sum output).
	binaryChecksum := unsafeslice.AsString(h.Sum(nil))

	fmt.Printf("%x\n", binaryChecksum)

	// Output:
	// 38d1334144987bf4
}

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
