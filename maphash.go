// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.14
// +build !unsafe race

package unsafeslice

import (
	"hash/maphash"
	"sync"
)

type hash64 = *maphash.Hash

func newHash() hash64 {
	return new(maphash.Hash)
}

// initHash is separate from newHash because the call to sync.once.Do
// confounds the inliner, which then induces an extra allocation
// for the hasher.
func initHash(h hash64) {
	seed.once.Do(func() {
		seed.seed = maphash.MakeSeed()
	})
	h.SetSeed(seed.seed)
}

func disposeHash(hash64) {}

var seed struct {
	once sync.Once
	seed maphash.Seed
}
