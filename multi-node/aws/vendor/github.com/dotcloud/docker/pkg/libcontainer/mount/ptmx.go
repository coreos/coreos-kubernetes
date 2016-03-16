// +build linux

package mount

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer/console"
	"os"
	"path/filepath"
)

func SetupPtmx(rootfs, consolePath, mountLabel string) error {
	ptmx := filepath.Join(rootfs, "dev/ptmx")
	if err := os.Remove(ptmx); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink("pts/ptmx", ptmx); err != nil {
		return fmt.Errorf("symlink dev ptmx %s", err)
	}
	if consolePath != "" {
		if err := console.Setup(rootfs, consolePath, mountLabel); err != nil {
			return err
		}
	}
	return nil
}
