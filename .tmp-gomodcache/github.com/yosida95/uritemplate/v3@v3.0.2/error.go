// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"fmt"
)

func errorf(pos int, format string, a ...interface{}) error {
	msg := fmt.Sprintf(format, a...)
	return fmt.Errorf("uritemplate:%d:%s", pos, msg)
}
