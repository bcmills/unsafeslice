// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.14

package unsafeslice

import (
	"hash/maphash"
	"sync"
)

var seed struct {
	once sync.Once
	seed maphash.Seed
}

func newHash() *maphash.Hash {
	seed.once.Do(func() {
		seed.seed = maphash.MakeSeed()
	})

	h := new(maphash.Hash)
	h.SetSeed(seed.seed)
	return h
}
