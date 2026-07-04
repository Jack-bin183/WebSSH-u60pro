//go:build !linux || !cgo

package diagefs

import "fmt"

func diagOpen(path string) error {
	return fmt.Errorf("串号更新需要在 Linux 设备上使用 CGO 构建，并且系统可加载 libdiag.so")
}

func diagInitDCI() error {
	return fmt.Errorf("串号更新需要在 Linux 设备上使用 CGO 构建，并且系统可加载 libdiag.so")
}

func diagClose() {}

func efsWriteFile(path string, data []byte) error {
	return fmt.Errorf("串号更新需要在 Linux 设备上使用 CGO 构建，并且系统可加载 libdiag.so")
}
