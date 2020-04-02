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
// The caller must ensure that src meets the alignment requirements for dst.
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
