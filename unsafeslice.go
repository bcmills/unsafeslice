// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package unsafeslice provides generic functions for unsafe transformations on
// slice values.
package unsafeslice

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/bcmills/unsafeslice/internal/eventually"
)

// SetAt sets dst, which must be a non-nil pointer to a variable of a slice
// type, to a slice of length and capacity n located at p.
//
// The caller must ensure that p meets the alignment requirements for dst, and
// that the allocation to which p points contains at least n contiguous
// elements.
//
// This implements one possible API for https://golang.org/issue/19367
// and https://golang.org/issue/13656.
func SetAt(dst interface{}, p unsafe.Pointer, n int) {
	dv := reflect.ValueOf(dst)
	dt := dv.Type()
	if dt.Kind() != reflect.Ptr || dt.Elem().Kind() != reflect.Slice {
		panic(fmt.Sprintf("SetAt with dst type %T; need *[]T", dst))
	}

	hdr := (*reflect.SliceHeader)(unsafe.Pointer(dv.Pointer()))

	// Safely zero any existing slice at *dst, ensuring that it never contains an
	// invalid slice.
	hdr.Len = 0
	hdr.Cap = 0

	// Now set the slice to point to p, then expand the cap and length,
	// again ensuring that the slice is always valid.
	hdr.Data = uintptr(p)
	hdr.Cap = n
	hdr.Len = n
}

// ConvertAt sets dst, which must be a non-nil pointer to a variable of a slice
// type, to a slice that refers to the same memory region as the slice src,
// but possibly at a different type.
//
// The caller must ensure that src meets the alignment requirements for dst, and
// that the length and capacity of src are integer multiples of the element size
// of dst.
//
// This implements one possible API for https://golang.org/issue/38203.
func ConvertAt(dst, src interface{}) {
	sv := reflect.ValueOf(src)
	st := sv.Type()
	if st.Kind() != reflect.Slice {
		panic(fmt.Sprintf("ConvertAt with src type %T; need []T", src))
	}

	dv := reflect.ValueOf(dst)
	dt := dv.Type()
	if dt.Kind() != reflect.Ptr || dt.Elem().Kind() != reflect.Slice {
		panic(fmt.Sprintf("ConvertAt with dst type %T; need *[]T", dst))
	}

	srcElemSize := st.Elem().Size()
	capBytes := uintptr(sv.Len()) * srcElemSize
	lenBytes := uintptr(sv.Cap()) * srcElemSize

	dstElemSize := dt.Elem().Elem().Size()

	if capBytes%dstElemSize != 0 {
		panic(fmt.Sprintf("ConvertAt: src capacity (%d bytes) is not a multiple of dst element size (%v: %d bytes)", capBytes, dt.Elem().Elem(), dstElemSize))
	}
	dstCap := capBytes / dstElemSize
	if int(dstCap) < 0 || uintptr(int(dstCap)) != dstCap {
		panic(fmt.Sprintf("ConvertAt: dst capacity (%d) overflows int", dstCap))
	}

	if lenBytes%dstElemSize != 0 {
		panic(fmt.Sprintf("ConvertAt: src length (%d bytes) is not a multiple of dst element size (%v: %d bytes)", lenBytes, dt.Elem().Elem(), dstElemSize))
	}
	dstLen := lenBytes / dstElemSize
	if int(dstLen) < 0 || uintptr(int(dstLen)) != dstLen {
		panic(fmt.Sprintf("ConvertAt: dst length (%d) overflows int", dstLen))
	}

	hdr := (*reflect.SliceHeader)(unsafe.Pointer(dv.Pointer()))

	// Safely zero any existing slice at *dst, ensuring that it never contains an
	// invalid slice.
	hdr.Len = 0
	hdr.Cap = 0

	// Now set the slice to point to src, then expand the cap and length,
	// again ensuring that the slice is always valid.
	hdr.Data = uintptr(unsafe.Pointer(sv.Pointer()))
	hdr.Cap = int(dstCap)
	hdr.Len = int(dstLen)
}

// OfString returns a slice that refers to the data backing the string s.
//
// The caller must ensure that the contents of the slice are never mutated.
//
// Programs that use unsafeslice.OfString should be tested under the race
// detector to flag erroneous mutations.
func OfString(s string) []byte {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)

	var b []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)

	maybeDetectMutations(b)
	return b
}

// AsString returns a string that refers to the data backing the slice s.
//
// The caller must ensure that the contents of the slice are never again
// mutated, and that its memory either is managed by the Go garbage collector or
// remains valid for the remainder of this process's lifetime.
//
// Programs that use unsafeslice.AsString should be tested under the race
// detector to flag erroneous mutations.
func AsString(b []byte) string {
	p := unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&b)).Data)

	var s string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	hdr.Data = uintptr(p)
	hdr.Len = len(b)

	maybeDetectMutations(b)
	return s
}

// maybeDetectMutations makes a best effort to detect mutations and lifetime
// errors on the slice b. It is most effective when run under the race detector.
func maybeDetectMutations(b []byte) {
	if safetyReduced() || len(b) == 0 {
		return
	}

	checksum := new(uint64)

	h := newHash()
	h.Write(b)
	*checksum = h.Sum64()

	if raceEnabled {
		// Start a goroutine that reads from the slice and does not have a
		// happens-before relationship with any other event in the program.
		//
		// Even if the goroutine is scheduled and runs to completion immediately, if
		// anything ever mutates the slice the race detector should report it as a
		// read/write race. The erroneous writer should be easy to identify from the
		// race report.
		go func() {
			h := newHash()
			h.Write(b)
			if *checksum != h.Sum64() {
				panic(fmt.Sprintf("mutation detected in string at address 0x%012x", &b[0]))
			}
		}()
	}

	// We can't set a finalizer on the slice contents itself, since we don't know
	// how it was allocated (or even whether it is owned by the Go runtime).
	// Instead, we use a finalizer on the checksum allocation to make a best
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
	eventually.SetFinalizer(checksum, func(checksum *uint64) {
		h := newHash()
		h.Write(b)
		if *checksum != h.Sum64() {
			panic(fmt.Sprintf("mutation detected in string at address 0x%012x", &b[0]))
		}
	})
}

// ReduceSafety may make the unsafeslice package even less safe,
// but more efficient.
//
// ReduceSafety has no effect when the race detector is enabled:
// the available safety checks are always enabled under the race detector,
// and will generally produce clearer diagnostics.
func ReduceSafety() {
	if !raceEnabled {
		atomic.StoreInt32(&safetyReducedFlag, 1)
	}
}

var safetyReducedFlag int32 = 0

func safetyReduced() bool {
	return atomic.LoadInt32(&safetyReducedFlag) != 0
}
