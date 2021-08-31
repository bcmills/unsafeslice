// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unsafeslice_test

import (
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"runtime"
	"testing"
	"unsafe"

	"github.com/bcmills/unsafeslice"
)

// asCPointer returns b as a C-style pointer and length
func asCPointer(b []byte) (*int8, int) {
	if len(b) == 0 {
		return nil, 0
	}
	return (*int8)(unsafe.Pointer(&b[0])), len(b)
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

func TestConvertAt(t *testing.T) {
	u32 := []uint32{0x00102030, 0x40506070}[:1]
	var b []byte
	unsafeslice.ConvertAt(&b, u32)

	if want := len(u32) * 4; len(b) != want {
		t.Errorf("ConvertAt(_, %x): length = %v; want %v", u32, len(b), want)
	}
	if want := cap(u32) * 4; cap(b) != want {
		t.Errorf("ConvertAt(_, %x): capacity = %v; want %v", u32, cap(b), want)
	}
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

	// This is safe because the contract for an io.Writer requires that
	// “Write must not modify the slice data, even temporarily.”
	h.Write(b)

	fmt.Printf("%x\n", h.Sum(nil))

	// Output:
	// 38d1334144987bf4
}

func ExampleAsString() {
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

func TestStringAllocs(t *testing.T) {
	t.Run("OfString", func(t *testing.T) {
		s := "Hello, world!"

		var b []byte
		avg := testing.AllocsPerRun(1000, func() {
			b = unsafeslice.OfString(s)
		})
		runtime.KeepAlive(b)

		if avg > 0.01+maxStringAllocs {
			t.Errorf("unsafeslice.OfString made %v allocations; want %d", avg, maxStringAllocs)
		}
	})

	t.Run("AsString", func(t *testing.T) {
		b := []byte("Hello, world!")

		var s string
		avg := testing.AllocsPerRun(1000, func() {
			s = unsafeslice.AsString(b)
		})
		runtime.KeepAlive(s)

		if avg > 0.01+maxStringAllocs {
			t.Errorf("unsafeslice.OfString made %v allocations; want %d", avg, maxStringAllocs)
		}
	})
}

func BenchmarkOfString(b *testing.B) {
	in := "Hello, world!"
	var out []byte
	for n := b.N; n > 0; n-- {
		out = unsafeslice.OfString(in)
	}
	runtime.KeepAlive(out)
}

func BenchmarkAsString(b *testing.B) {
	in := []byte("Hello, world!")
	var out string
	for n := b.N; n > 0; n-- {
		out = unsafeslice.AsString(in)
	}
	runtime.KeepAlive(out)
}
