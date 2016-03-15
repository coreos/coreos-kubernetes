// +build linux,amd64

package devmapper

import (
	"github.com/dotcloud/docker/daemon/graphdriver/graphtest"
	"testing"
)

func init() {
	// Reduce the size the the base fs and loopback for the tests
	DefaultDataLoopbackSize = 300 * 1024 * 1024
	DefaultMetaDataLoopbackSize = 200 * 1024 * 1024
	DefaultBaseFsSize = 300 * 1024 * 1024
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestDevmapperSetup and TestDevmapperTeardown
func TestDevmapperSetup(t *testing.T) {
	graphtest.GetDriver(t, "devicemapper")
}

func TestDevmapperCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "devicemapper")
}

func TestDevmapperCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "devicemapper")
}

func TestDevmapperCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "devicemapper")
}

func TestDevmapperTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
