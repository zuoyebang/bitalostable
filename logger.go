// Copyright 2011 The LevelDB-Go and Pebble and Bitalostored Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package bitalostable

import (
	"github.com/zuoyebang/bitalostable/internal/base"
)

// Logger defines an interface for writing log messages.
type Logger = base.Logger

var DefaultLogger = base.DefaultLogger
