// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package unsafeslice provides generic functions for unsafe transformations on
// slice values.
package unsafeslice

import (
	"fmt"
	"reflect"
	"unsafe"
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
	capBytes := uintptr(sv.Cap()) * srcElemSize
	lenBytes := uintptr(sv.Len()) * srcElemSize

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
// Programs that use OfString should be tested under the race detector to flag
// erroneous mutations.
//
// Programs that have been adequately tested and shown to be safe may be
// recompiled with the "unsafe" tag to significantly reduce the overhead of this
// function, at the cost of reduced safety checks. Programs built under the race
// detector always have safety checks enabled, even when the "unsafe" tag is
// set.
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
// Programs that use AsString should be tested under the race detector to flag
// erroneous mutations.
//
// Programs that have been adequately tested and shown to be safe may be
// recompiled with the "unsafe" tag to significantly reduce the overhead of this
// function, at the cost of reduced safety checks. Programs built under the race
// detector always have safety checks enabled, even when the "unsafe" tag is
// set.
func AsString(b []byte) string {
	p := unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&b)).Data)

	var s string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	hdr.Data = uintptr(p)
	hdr.Len = len(b)

	maybeDetectMutations(b)
	return s
}
