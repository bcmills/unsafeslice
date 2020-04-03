// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package unsafeslice_test

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/bcmills/unsafeslice"
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
			src:  []byte("foobar")[:4],
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
