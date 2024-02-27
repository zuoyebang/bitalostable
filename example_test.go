// Copyright 2020 The LevelDB-Go and Pebble and Bitalostored Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package bitalostable_test

import (
	"fmt"
	"log"

	"github.com/zuoyebang/bitalostable"
	"github.com/zuoyebang/bitalostable/vfs"
)

func Example() {
	db, err := bitalostable.Open("", &bitalostable.Options{FS: vfs.NewMem()})
	if err != nil {
		log.Fatal(err)
	}
	key := []byte("hello")
	if err := db.Set(key, []byte("world"), bitalostable.Sync); err != nil {
		log.Fatal(err)
	}
	value, closer, err := db.Get(key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s %s\n", key, value)
	if err := closer.Close(); err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
	// Output:
	// hello world
}
