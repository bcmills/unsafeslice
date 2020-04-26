// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !go1.14
// +build !unsafe race

package unsafeslice

import (
	"hash"
	"hash/fnv"
	"sync"
)

type hash64 = hash.Hash64

// hashPool stores unused hashers.
//
// We need a sync.Pool because the escape analysis in 1.13 and earlier isn't
// clever enough to avoid heap-allocating an FNV hasher, causing
// TestStringAllocs to fail.
var hashPool = sync.Pool{
	New: func() interface{} {
		return fnv.New64a()
	},
}

func newHash() hash64 {
	return hashPool.Get().(hash64)
}

func initHash(h hash64) {
	h.Reset()
}

func disposeHash(h hash64) {
	hashPool.Put(h)
}
