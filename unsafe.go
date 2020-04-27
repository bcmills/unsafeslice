// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build unsafe,!race

package unsafeslice

// This file contains declarations for “extra unsafe” mode,
// which disables mutation checks for string functions.

// maybeDetectMutations makes no attempt whatsoever to detect mutations and
// lifetime errors on the passed-in slice.
func maybeDetectMutations([]byte) {}
