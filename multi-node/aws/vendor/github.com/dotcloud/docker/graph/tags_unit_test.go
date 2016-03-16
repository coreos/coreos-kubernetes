package graph

import (
	"bytes"
	"github.com/dotcloud/docker/daemon/graphdriver"
	_ "github.com/dotcloud/docker/daemon/graphdriver/vfs" // import the vfs driver so it is used in the tests
	"github.com/dotcloud/docker/image"
	"github.com/dotcloud/docker/utils"
	"github.com/dotcloud/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
	"io"
	"os"
	"path"
	"testing"
)

const (
	testImageName = "myapp"
	testImageID   = "foo"
)

func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}

func mkTestTagStore(root string, t *testing.T) *TagStore {
	driver, err := graphdriver.New(root)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewGraph(root, driver)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewTagStore(path.Join(root, "tags"), graph)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img := &image.Image{ID: testImageID}
	// FIXME: this fails on Darwin with:
	// tags_unit_test.go:36: mkdir /var/folders/7g/b3ydb5gx4t94ndr_cljffbt80000gq/T/docker-test569b-tRunner-075013689/vfs/dir/foo/etc/postgres: permission denied
	if err := graph.Register(nil, archive, img); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(testImageName, "", testImageID, false); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestLookupImage(t *testing.T) {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store := mkTestTagStore(tmp, t)
	defer store.graph.driver.Cleanup()

	if img, err := store.LookupImage(testImageName); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}
	if img, err := store.LookupImage(testImageName + ":" + DEFAULTTAG); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := store.LookupImage(testImageName + ":" + "fail"); err == nil {
		t.Errorf("Expected error, none found")
	} else if img != nil {
		t.Errorf("Expected 0 image, 1 found")
	}

	if img, err := store.LookupImage("fail:fail"); err == nil {
		t.Errorf("Expected error, none found")
	} else if img != nil {
		t.Errorf("Expected 0 image, 1 found")
	}

	if img, err := store.LookupImage(testImageID); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := store.LookupImage(testImageName + ":" + testImageID); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}
}
