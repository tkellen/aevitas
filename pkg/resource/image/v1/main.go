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

// GroupName is the group name used in this package.
const GroupName = "image"

type imageSpec struct {
	ID     string
	Name   string
	Widths []int
}

func (img *imageSpec) Validate() error {
	var errs []string
	if img.ID == "" {
		errs = append(errs, "id must be defined")
	}
	if img.Name == "" {
		errs = append(errs, "name must be defined")
	}
	if len(img.Widths) == 0 {
		errs = append(errs, "at least one size must be provided")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (img *imageSpec) Scope(fs billy.Filesystem) (billy.Filesystem, error) {
	return fs.Chroot(filepath.Join(GroupName, img.ID))
}

func (img *imageSpec) Current(fs billy.Filesystem) bool {
	var widths []int
	for _, width := range img.Widths {
		if stat, _ := fs.Stat(strconv.Itoa(width)); stat != nil && stat.Size() != 0 {
			continue
		}
		widths = append(widths, width)
	}
	return len(widths) == 0
}

func (img *imageSpec) Render(ctx context.Context, render func(int) error) error {
	// Compute all sizes simultaneously. This may need to be gated further but
	// attempting to do so from the caller first.
	eg, egCtx := errgroup.WithContext(ctx)
	for _, width := range img.Widths {
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
