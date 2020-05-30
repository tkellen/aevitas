package v1

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"golang.org/x/sync/errgroup"
	"path/filepath"
	"strconv"
	"strings"
)

type imageSpec struct {
	Name   string
	Widths []int
}

func (is *imageSpec) Validate() error {
	var errs []string
	if is.Name == "" {
		errs = append(errs, "spec.name must be defined")
	}
	if len(is.Widths) == 0 {
		errs = append(errs, "spec.size must be defined as an array")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (is *imageSpec) Scope(fs billy.Filesystem, name string) (billy.Filesystem, error) {
	return fs.Chroot(filepath.Join("asset", "image", name))
}

func (is *imageSpec) Current(fs billy.Filesystem) bool {
	var widths []int
	for _, width := range is.Widths {
		if stat, _ := fs.Stat(strconv.Itoa(width)); stat != nil && stat.Size() != 0 {
			continue
		}
		widths = append(widths, width)
	}
	return len(widths) == 0
}

func (is *imageSpec) Render(ctx context.Context, render func(int) error) error {
	// Compute all sizes simultaneously. This may need to be gated further but
	// attempting to do so from the caller first.
	eg, egCtx := errgroup.WithContext(ctx)
	for _, width := range is.Widths {
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
