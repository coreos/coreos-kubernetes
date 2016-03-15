package blobutil

type NamedBufferList []*NamedBuffer

func (bufList NamedBufferList) WriteToFiles(dirPath string) error {
	for _, buffer := range bufList {
		if err := buffer.WriteToFile(dirPath); err != nil {
			return err
		}
	}
	return nil
}

func (bufList NamedBufferList) ReadFromFiles(dirPath string) error {
	for _, buffer := range bufList {
		if err := buffer.ReadFromFile(dirPath); err != nil {
			return err
		}
	}
	return nil
}

func (bufList NamedBufferList) EncodeBuffers() error {
	for _, buffer := range bufList {
		if err := buffer.Encode(); err != nil {
			return err
		}
	}
	return nil
}

func (bufList NamedBufferList) TemplateBuffers(data interface{}) error {
	for _, buffer := range bufList {
		if err := buffer.Template(data); err != nil {
			return err
		}
	}

	return nil
}
