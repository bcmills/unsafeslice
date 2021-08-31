// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package unsafeslice

import (
	"fmt"
	"unsafe"
	"reflect"
)

// The CChar constraint matches any type that could be C.char on some platform.
type CChar interface {
	~int8 | ~uint8
}

// OfCString returns a slice that aliases the null-terminated (“C-style”) string
// pointed to by p. The length and capacity of the returned slice do not include
// the trailing null-byte.
func OfCString[T CChar](p *T) []byte {
	return unsafe.Slice((*byte)((unsafe.Pointer)(p)), StrLen(p))
}

// StrLen returns the length of the 0-terminated (“C-style”) array to which p
// points: that is, the number of nonzero elements before the first zero
// element.
func StrLen[T comparable](p *T) int {
	var zero T
	n := 0
	for *p != zero {
		n++
		if n < 0 {
			panic("length overflow")
		}
		p = (*T)(unsafe.Add(unsafe.Pointer(p), unsafe.Sizeof(*p)))
	}
	return n
}

// The SliceOf constraint matches any slice type with element type E.
type SliceOf[E any] interface {
	~[]E
}

// ConvertTo returns a slice that refers to the same memory region as src,
// but as a slice of DstElem instead of a slice of SrcElem.
//
// The caller must ensure that src meets the alignment requirements for dst, and
// that the length and capacity of src are integer multiples of the element size
// of dst.
//
// This implements one possible API for https://golang.org/issue/38203.
func ConvertTo[DstElem any, SrcElem any, Src SliceOf[SrcElem]](src Src) ([]DstElem) {
	srcElemSize := unsafe.Sizeof(src[0])
	capBytes := uintptr(cap(src)) * srcElemSize
	lenBytes := uintptr(len(src)) * srcElemSize

	var dst []DstElem
	dstElemSize := unsafe.Sizeof(dst[0])
	if capBytes%dstElemSize != 0 {
		panic(fmt.Sprintf("ConvertTo: src capacity (%d bytes) is not a multiple of dst element size (%d bytes)", capBytes, dstElemSize))
	}
	dstCap := capBytes / dstElemSize
	if int(dstCap) < 0 || uintptr(int(dstCap)) != dstCap {
		panic(fmt.Sprintf("ConvertTo: dst capacity (%d) overflows int", dstCap))
	}

	if lenBytes%dstElemSize != 0 {
		panic(fmt.Sprintf("ConvertTo: src length (%d bytes) is not a multiple of dst element size (%d bytes)", lenBytes, dstElemSize))
	}
	dstLen := lenBytes / dstElemSize
	if int(dstLen) < 0 || uintptr(int(dstLen)) != dstLen {
		panic(fmt.Sprintf("ConvertTo: dst length (%d) overflows int", dstLen))
	}

	// We can't use &src[0] or &(src[:1][0]) here, because cap(src) may be 0 even
	// if src is non-nil.
	srcHdr := (*reflect.SliceHeader)(unsafe.Pointer(&src))
	return unsafe.Slice((*DstElem)(unsafe.Pointer(srcHdr.Data)), dstCap)[:dstLen]
}
