package image

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"golang.org/x/sync/errgroup"
	"strconv"
	"strings"
)

type imageSpec struct {
	Title  string
	Widths []int
	Path   string
	Domain string
}

func (s *imageSpec) validate() error {
	var errs []string
	if s.Title == "" {
		errs = append(errs, "title must be defined")
	}
	if s.Path == "" {
		errs = append(errs, "path must be defined")
	}
	if len(s.Widths) == 0 {
		errs = append(errs, "widths must be defined as an array")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (s *imageSpec) scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot(s.Path)
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
	// Compute all sizes simultaneously. This may need to be gated further but
	// attempting to do so from the caller first.
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
