package blobutil

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"
)

type NamedBuffer struct {
	bytes.Buffer
	Name string
}

func (buf *NamedBuffer) Encode() error {
	//Copy existing data
	inBytes := make([]byte, buf.Len())
	copy(inBytes, buf.Bytes())
	in := bytes.NewBuffer(inBytes)

	buf.Reset()

	b64Writer := base64.NewEncoder(base64.StdEncoding, buf)
	defer b64Writer.Close()

	gzWriter, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("buffer %s : failed creating gzip context: %v", buf.Name, err)
	}
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, in); err != nil {
		return fmt.Errorf("buffer %s: failed reading input: %v", buf.Name, err)
	}

	return nil
}

func (buf *NamedBuffer) Template(data interface{}) error {
	tmpl, err := template.New(buf.Name).Parse(buf.String())
	if err != nil {
		return fmt.Errorf("buffer %s: error templating : %v", buf.Name, err)
	}

	buf.Reset()

	if err := tmpl.Execute(buf, data); err != nil {
		return fmt.Errorf("buffer %s: error templating: %v", buf.Name, err)
	}

	return nil
}

func (buf *NamedBuffer) WriteToFile(dirPath string) error {
	path := filepath.Join(dirPath, buf.Name)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("error opening %s : %v", path, err)
	}
	defer out.Close()
	if _, err := buf.WriteTo(out); err != nil {
		return fmt.Errorf("error writing %s : %v", path, err)
	}

	return nil
}

func (buf *NamedBuffer) ReadFromFile(dirPath string) error {
	buf.Reset()

	path := filepath.Join(dirPath, buf.Name)
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening %s : %v", path, err)
	}
	defer in.Close()

	if _, err := buf.ReadFrom(in); err != nil {
		return fmt.Errorf("error reading %s : %v", path, err)
	}

	return nil
}
