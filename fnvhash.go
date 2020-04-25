// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !go1.14

package unsafeslice

import (
	"hash"
	"hash/fnv"
)

func newHash() hash.Hash64 {
	return fnv.New64a()
}
