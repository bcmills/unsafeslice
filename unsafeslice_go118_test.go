// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package unsafeslice_test

import (
	"fmt"

	"github.com/bcmills/unsafeslice"
)

func ExampleOfCString() {
	p, _ := asCPointer([]byte("Hello, world!\x00"))

	b := unsafeslice.OfCString(p)

	fmt.Printf("%s\n", b)

	// Output:
	// Hello, world!
}
