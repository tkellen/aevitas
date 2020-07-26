package asset

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/tkellen/aevitas/pkg/manifest"
	"golang.org/x/sync/errgroup"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

type imageSpec struct {
	Title       string
	Description string
	Body        string
	Widths      []int
	Href        string
	Domain      string
}

func (s *imageSpec) validate() error {
	var errs []string
	if s.Title == "" {
		errs = append(errs, "title must be defined")
	}
	if len(s.Widths) == 0 {
		errs = append(errs, "widths must be defined as an array")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (s *imageSpec) current(fs billy.Filesystem) bool {
	var widths []int
	for _, width := range s.Widths {
		if stat, _ := fs.Stat(strconv.Itoa(width)); stat != nil && stat.Size() != 0 {
			continue
		}
		widths = append(widths, width)
	}
	return len(widths) == 0
}

func (s *imageSpec) render(ctx context.Context, render func(int) error) error {
	// Compute all sizes simultaneously.
	eg, egCtx := errgroup.WithContext(ctx)
	for _, width := range s.Widths {
		width := width
		eg.Go(func() error {
			// Detect cancellation.
			if egCtx.Err() != nil {
				return egCtx.Err()
			}
			return render(width)
		})
	}
	return eg.Wait()
}

func reader(m *manifest.Manifest, source billy.Filesystem) (io.ReadCloser, error) {
	return source.Open(fmt.Sprintf("%s", m.Meta.File))
}

func bytes(m *manifest.Manifest, source billy.Filesystem) ([]byte, error) {
	reader, fetchErr := reader(m, source)
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
