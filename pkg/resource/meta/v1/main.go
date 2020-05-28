package v1

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// GroupName is the group name used in this package.
const GroupName = "Meta"

type Meta struct {
	File string
}

func (m *Meta) DataReader(_ context.Context) (io.ReadCloser, error) {
	return os.Open(fmt.Sprintf("/home/tkellen/memorybox/%s", m.File))
}

func (m *Meta) DataBytes(ctx context.Context) ([]byte, error) {
	reader, fetchErr := m.DataReader(ctx)
	if fetchErr != nil {
		return nil, fetchErr
	}
	defer reader.Close()
	data, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		return nil, readErr
	}
	return data, nil
}
