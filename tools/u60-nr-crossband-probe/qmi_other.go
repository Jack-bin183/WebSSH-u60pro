//go:build !linux

package main

import (
	"context"
	"fmt"
)

func runRawNRScan(context.Context, []uint32, uint32, func(qmiEvent)) error {
	return fmt.Errorf("raw QRTR scanning is only available on Linux")
}
