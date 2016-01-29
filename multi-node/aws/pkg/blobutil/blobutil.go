package blobutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func MustTarAndCompressDirectory(basepath, path string) []byte {
	buf := &bytes.Buffer{}

	b64Writer := base64.NewEncoder(base64.StdEncoding, buf)
	defer b64Writer.Close()

	gzWriter, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
	if err != nil {
		stderr("Error creating gzip writer: %v", err)
		os.Exit(1)
	}
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	tarHandler := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			//Terminate on error
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(basepath, path)
		if err != nil {
			return err
		}

		hdr.Name = relPath

		if err = tarWriter.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tarWriter, f); err != nil {
				return err
			}
		}

		return nil
	}

	if err := filepath.Walk(path, tarHandler); err != nil {
		stderr("Error tar-ing directory %s: %v", path, err)
		os.Exit(1)
	}
	return buf.Bytes()
}

func MustReadAndCompressFile(loc string) []byte {
	f, err := os.Open(loc)
	if err != nil {
		stderr("Failed opening file %s: %v", loc, err)
		os.Exit(1)
	}
	defer f.Close()

	buf := &bytes.Buffer{}

	b64Writer := base64.NewEncoder(base64.StdEncoding, buf)
	defer b64Writer.Close()

	gzWriter, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
	if err != nil {
		stderr("Failed creating gzip context: %v", err)
		os.Exit(1)
	}
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, f); err != nil {
		stderr("Failed reading file %s: %v", loc, err)
		os.Exit(1)
	}

	return buf.Bytes()
}

func stderr(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
