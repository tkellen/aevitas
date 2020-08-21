// Package manifest provides a basis for describing a resource of any type.
package manifest

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	json "github.com/json-iterator/go"
	"github.com/lestrrat-go/strftime"
	hash "github.com/minio/sha256-simd"
	"github.com/tidwall/sjson"
	"github.com/tkellen/aevitas/internal/selector"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Manifest represents data that is used to instantiate a resource.
type Manifest struct {
	// Selector uniquely defines the manifest and provides the basis for
	// relating manifests to one another.
	Selector *selector.Selector
	// Meta provides additional information about the resource.
	Meta *Meta
	// GenerateManifests describes how one manifest can generate others.
	GenerateManifests []*Generator
	// Body contains the textual content of a manifest.
	Body string
	// Spec contains details specific to the kind/group/version of the manifest.
	Spec json.RawMessage
	// Source indicates where the manifest originated.
	Source string
	// Raw is the raw data that produced the manifest
	Raw []byte
	// Hash is the sha256 hash of the raw content of the manifest. This provides
	// the basis for cache busting of generated resources.
	Hash string
}

// Validate does just what you think it does.
func (m *Manifest) Validate() error {
	for _, generator := range m.GenerateManifests {
		if err := generator.Validate(); err != nil {
			return fmt.Errorf("%s: %w", generator.Name, err)
		}
	}
	if err := m.Meta.validate(); err != nil {
		return err
	}
	return nil
}

// String returns an identifier referencing where the manifest originated.
func (m *Manifest) String() string {
	if m.Source != "stream" {
		return m.Source
	}
	return fmt.Sprintf("%s (%s)", m.Selector, m.Source)
}

// UnmarshalJSON does just what you think it does.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	var temp struct {
		Kind              string
		Group             string
		Version           string
		Name              string
		Namespace         string
		Meta              *Meta
		GenerateManifests []*Generator
		Body              string
		Spec              json.RawMessage
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}
	s, err := selector.New(fmt.Sprintf("%s/%s/%s/%s/%s", temp.Kind, temp.Group, temp.Version, temp.Namespace, temp.Name))
	if err != nil {
		return err
	}
	*m = Manifest{
		Selector:          s,
		Meta:              temp.Meta,
		GenerateManifests: temp.GenerateManifests,
		Spec:              temp.Spec,
		Body:              temp.Body,
	}
	return nil
}

// Date returns a native time from the deconstructed form stored in metadata.
func (m *Manifest) PublishAt() time.Time {
	if m.Meta.PublishAt == nil {
		return time.Time{}
	}
	return time.Date(
		m.Meta.PublishAt.Year,
		time.Month(m.Meta.PublishAt.Month),
		m.Meta.PublishAt.Day,
		m.Meta.PublishAt.Hours,
		m.Meta.PublishAt.Minutes,
		m.Meta.PublishAt.Seconds,
		0,
		time.UTC,
	)
}

// PublishMonthDay returns a native time from the deconstructed form stored in
// metadata. It only uses the month/day for the time. This is used for indexing
// purposes (to find manifests that were published on the same month and day
// across multiple years).
func (m *Manifest) PublishMonthDay() time.Time {
	if m.Meta.PublishAt == nil {
		return time.Time{}
	}
	return time.Date(
		0,
		time.Month(m.Meta.PublishAt.Month),
		m.Meta.PublishAt.Day,
		0,
		0,
		0,
		0,
		time.UTC,
	)
}

// IsLive determines if the manifest is considered published.
func (m *Manifest) IsLive() bool {
	if m.Meta == nil {
		return false
	}
	if !m.Meta.Live {
		return false
	}
	if !m.PublishAt().IsZero() {
		return time.Now().After(m.PublishAt())
	}
	return true
}

// Greater determines if the receiver manifest was published after the compared.
// In the absence of a publish date, fall back to alphanumeric sorting by ID.
func (m *Manifest) Greater(compare *Manifest) bool {
	if compare == nil || compare.Meta == nil {
		return false
	}
	if m.PublishAt().IsZero() || compare.PublishAt().IsZero() {
		return m.Selector.ID() > compare.Selector.ID()
	}
	return m.PublishAt().After(compare.PublishAt())
}

// Less determines if the receiver manifest was published before the compared.
// In the absence of a publish date, fall back to alphanumeric sorting by ID.
func (m *Manifest) Less(compare *Manifest) bool {
	if compare == nil || compare.Meta == nil {
		return false
	}
	if m.PublishAt().IsZero() || compare.PublishAt().IsZero() {
		return m.Selector.ID() < compare.Selector.ID()
	}
	return m.PublishAt().Before(compare.PublishAt())
}

// Equal determines if the receiver manifest was published at the same time as
// the compared. In the absence of a publish date, fall back to alphanumeric
// sorting by ID.
func (m *Manifest) Equal(compare *Manifest) bool {
	if compare == nil || compare.Meta == nil {
		return false
	}
	if m.PublishAt().IsZero() || compare.PublishAt().IsZero() {
		return true
	}
	return m.PublishAt().Equal(compare.PublishAt())
}

func (m *Manifest) Title() string {
	return m.Meta.Title
}

func (m *Manifest) Href() string {
	if m.Meta.HrefPrefix == "" {
		return m.Meta.Href
	}
	if m.Meta.Href == "" {
		return m.Meta.HrefPrefix
	}
	return path.Join(m.Meta.HrefPrefix, m.Meta.Href)
}

// Import describes a manifest that is required to render the parent.
type Import struct {
	Name       string
	Single     bool
	IsTemplate bool
	Manifests  []*Manifest
}

// ResolveStaticImports converts all imports selectors into manifests using the
// supplied index.
func (m *Manifest) ResolveStaticImports(index *Index) ([]*Import, error) {
	var associated []*Import
	for _, toImport := range m.Meta.Imports {
		expanded, err := toImport.Resolve(index)
		if err != nil {
			return nil, err
		}
		associated = append(associated, &Import{
			Name:       toImport.Name,
			Single:     !toImport.Selector.IsWildcard(),
			IsTemplate: toImport.Selector.KGV == "html/template/v1",
			Manifests:  expanded,
		})
	}
	return associated, nil
}

// ResolveDynamicImports computes all imports selectors that need details from
// the manifest that is importing them to complete the work.
func (m *Manifest) ResolveDynamicImports(index *Index, context *Manifest) ([]*Import, error) {
	var associated []*Import
	for _, toImport := range m.Meta.ImportsDynamic {
		expanded, err := toImport.Resolve(index, context)
		if err != nil {
			return nil, err
		}
		associated = append(associated, &Import{
			Name:       toImport.Name,
			Single:     !toImport.Selector.IsWildcard(),
			IsTemplate: toImport.Selector.KGV == "html/template/v1",
			Manifests:  expanded,
		})
	}
	return associated, nil
}

// New creates a manifest from a json-encoded byte array or a yaml-front-matter
// having byte array. If front-matter is found, the content below it is assigned
// to `.Spec.content` (overwriting any content that may be there).
func New(data []byte, source string) ([]*Manifest, error) {
	var manifest *Manifest
	var err error
	digest := hash.Sum256(data)
	body := append([]byte{}, data...)
	// Process front-matter, if any.
	if data, content, ok := frontmatter(body, []byte("<!--"), []byte("-->")); ok {
		if body, err = yaml.YAMLToJSON(data); err != nil {
			return nil, err
		}
		if len(content) > 0 {
			if body, err = sjson.SetBytes(body, "body", content); err != nil {
				return nil, err
			}
		}
	}
	if err = json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if manifest == nil {
		return nil, errors.New("invalid format")
	}
	manifest.Hash = hex.EncodeToString(digest[:])
	if manifest.Meta == nil {
		manifest.Meta = &Meta{}
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	if manifest.Meta.HrefPrefix != "" {
		parse, timeErr := strftime.New(manifest.Meta.HrefPrefix)
		if timeErr != nil {
			return nil, timeErr
		}
		manifest.Meta.HrefPrefix = parse.FormatString(manifest.PublishAt())
	}
	if manifest.Meta.Href != "" {
		parse, timeErr := strftime.New(manifest.Meta.Href)
		if timeErr != nil {
			return nil, timeErr
		}
		manifest.Meta.Href = parse.FormatString(manifest.PublishAt())
	}
	manifest.Raw = data
	manifest.Source = source
	var manifests []*Manifest
	if manifest.GenerateManifests != nil {
		for _, generator := range manifest.GenerateManifests {
			created, err := generator.Generate(manifest)
			if err != nil {
				return nil, fmt.Errorf("%s: generateManifests: %s: %w", manifest, generator.Name, err)
			}
			manifests = append(manifests, created...)
		}
	}
	return append(manifests, manifest), nil
}

// NewFromFile creates a manifest from a source file.
func NewFromFile(filepath string) ([]*Manifest, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	if path.Ext(filepath) == ".yml" {
		data, err = yaml.YAMLToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("%s: yaml to json failure: %w\n%s", filepath, err)
		}
	}
	manifest, newErr := New(data, filepath)
	if newErr != nil {
		return nil, fmt.Errorf("%s: %w", filepath, newErr)
	}
	return manifest, nil
}

type progressFn func(int, <-chan struct{})

// NewFromReader creates manifests from a provided reader taking the assumption
// that the reader contains one json-encoded manifest per line.
func NewFromReader(input io.Reader, watch progressFn) ([]*Manifest, error) {
	reader := bufio.NewReader(input)
	var docs [][]byte
	for {
		raw, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		docs = append(docs, bytes.TrimRight(raw, "\n"))
	}
	var manifests []*Manifest
	progress := make(chan struct{})
	if watch != nil {
		go watch(len(docs), progress)
	}
	for _, doc := range docs {
		if watch != nil {
			progress <- struct{}{}
		}
		results, err := New(doc, "stream")
		if err != nil {
			return nil, fmt.Errorf("\n---\n%s\n---\n: %w", doc, err)
		}
		manifests = append(manifests, results...)
	}
	close(progress)
	return manifests, nil
}

// NewFromDirs creates manifests from all files found in an array of supplied
// directories.
func NewFromDirs(dirs []string, watch progressFn) ([]*Manifest, error) {
	var files []string
	for _, dir := range dirs {
		if err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
			if f.IsDir() {
				if strings.HasPrefix(f.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(filepath.Base(f.Name()), ".") {
				return nil
			}
			files = append(files, path)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	var manifests []*Manifest
	progress := make(chan struct{})
	if watch != nil {
		go watch(len(files), progress)
	}
	for _, path := range files {
		if watch != nil {
			progress <- struct{}{}
		}
		results, err := NewFromFile(path)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, results...)
	}
	close(progress)
	return manifests, nil
}

func frontmatter(input []byte, openDelim []byte, closeDelim []byte) ([]byte, []byte, bool) {
	s := bytes.Index(input, openDelim)
	if s == -1 {
		return nil, nil, false
	}
	afterOpenDelim := input[s+len(openDelim):]
	closeDelimStart := bytes.Index(afterOpenDelim, closeDelim)
	if closeDelimStart == -1 {
		return nil, nil, false
	}
	data := afterOpenDelim[:closeDelimStart]
	content := afterOpenDelim[closeDelimStart+len(closeDelim):]
	return data, content, true
}
