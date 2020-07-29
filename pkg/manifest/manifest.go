// Package manifest provides a basis for describing a resource of any type.
package manifest

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	json "github.com/json-iterator/go"
	"github.com/lestrrat-go/strftime"
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
	Selector *selector.Selector
	// Meta provides optional additional information about the resource.
	Meta *Meta
	// Relations allows expressing relationships with other manifests.
	Relations []*Relation
	// Meta provides details about how to render the resource.
	Render *Render
	// GenerateManifests describes how one manifest can generate others.
	GenerateManifests []*Generator
	// Body contains the textual content of a manifest.
	Body string
	// Spec contains details specific to the kind/group/version of the manifest.
	Spec json.RawMessage
	// source indicates where the manifest originated.
	Source string
	// raw is the raw data that produced the manifest
	Raw []byte
}

// RenderManifest represents a manifest and those manifests which have been
// chosen to render it. This allows the same manifest to be consumed in many
// different configurations.
type RenderManifest struct {
	Manifest         *Manifest
	TemplateOverride Templates
}

// Validate does just what you think it does.
func (m *Manifest) Validate() error {
	for _, generator := range m.GenerateManifests {
		if err := generator.Validate(); err != nil {
			return fmt.Errorf("%s: %w", generator.Name, err)
		}
	}
	for _, related := range m.Relations {
		if err := related.validate(); err != nil {
			return err
		}
	}
	if err := m.Render.validate(); err != nil {
		return err
	}
	return nil
}

// String returns an identifier referencing where the manifest originated.
func (m *Manifest) String() string {
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
		Relations         []*Relation
		GenerateManifests []*Generator
		Render            *Render
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
		Render:            temp.Render,
		Relations:         temp.Relations,
		GenerateManifests: temp.GenerateManifests,
		Spec:              temp.Spec,
		Body:              temp.Body,
	}
	return nil
}

// Date returns a native time object from the deconstructed form.
func (m *Manifest) PublishAt() time.Time {
	if m.Meta.PublishAt == nil {
		return time.Time{}
	}
	pa := m.Meta.PublishAt
	return time.Date(pa.Year, time.Month(pa.Month), pa.Day, pa.Hours, pa.Minutes, pa.Seconds, 0, time.UTC)
}

func (m *Manifest) Title() string {
	if m.Meta.Title == m.Meta.TitleBase {
		return m.Meta.Title
	}
	if m.Meta.TitleBase == "" {
		return m.Meta.Title
	}
	if m.Meta.Title == "" {
		return m.Meta.TitleBase
	}
	return m.Meta.Title + " " +m.Meta.TitleBase
}

func (m *Manifest) Href() string {
	if m.Meta.HrefBase == "" {
		return m.Meta.Href
	}
	if m.Meta.Href == "" {
		return m.Meta.HrefBase
	}
	return path.Join(m.Meta.HrefBase, m.Meta.Href)
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

// ResolveChildren converts all children selectors into manifests using the
// supplied index.
func (m *Manifest) ResolveChildren(index *Index) ([]*RenderManifest, error) {
	var children []*RenderManifest
	for _, child := range m.Render.Children {
		resolvedChildren, relationErr := child.Relation.Resolve(index)
		if relationErr != nil {
			return nil, relationErr
		}
		for _, match := range resolvedChildren {
			children = append(children, &RenderManifest{
				Manifest:         match,
				TemplateOverride: child.Templates,
			})
		}
	}
	return children, nil
}

// AsImported describes a manifest as imported by name. It also allows consumers
// to know if the import was intended to produce multiple results or one.
type AsImported struct {
	Name       string
	Single     bool
	IsTemplate bool
	Manifests  []*Manifest
}

// ResolveStaticImports converts all imports selectors into manifests using the
// supplied index.
func (m *Manifest) ResolveStaticImports(index *Index) ([]*AsImported, error) {
	var imports []*AsImported
	for _, toImport := range m.Render.Imports {
		expanded, err := toImport.Resolve(index)
		if err != nil {
			return nil, err
		}
		imports = append(imports, &AsImported{
			Name:       toImport.Name,
			Single:     !toImport.Selector.IsWildcard(),
			IsTemplate: toImport.Selector.KGV == "html/template/v1",
			Manifests:  expanded,
		})
	}
	return imports, nil
}

// ResolveDynamicImports computes all imports selectors that need details from
// the manifest that is importing them to complete the work.
func (m *Manifest) ResolveDynamicImports(index *Index, context *Manifest) ([]*AsImported, error) {
	var imports []*AsImported
	for _, toImport := range m.Render.ImportsDynamic {
		expanded, err := toImport.Resolve(index, context)
		if err != nil {
			return nil, err
		}
		imports = append(imports, &AsImported{
			Name:       toImport.Name,
			Single:     !toImport.Selector.IsWildcard(),
			IsTemplate: toImport.Selector.KGV == "html/template/v1",
			Manifests:  expanded,
		})
	}
	return imports, nil
}

/*
func newFromCache(data []byte, cacheDir string) (*Manifest, error) {
	digest := hash.Sum256(data)
	cacheFile := filepath.Join(cacheDir,  hex.EncodeToString(digest[:]))
	gobReader, gobErr := os.Open(cacheFile)
	if gobErr != nil {
		return nil, gobErr
	}
	defer gobReader.Close()
	var manifest *Manifest
	if err := gob.NewDecoder(gobReader).Decode(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func saveToCache(manifest *Manifest, cacheDir string) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(manifest); err != nil {
		return err
	}
	digest := hash.Sum256(manifest.Raw)
	cacheFile := filepath.Join(cacheDir, hex.EncodeToString(digest[:]))
	return ioutil.WriteFile(cacheFile, buf.Bytes(), 0644)
}
*/

// New creates a manifest from a json-encoded byte array or a yaml-front-matter
// having byte array. If front-matter is found, the content below it is assigned
// to `.Spec.content` (overwriting any content that may be there).
func New(data []byte, source string) ([]*Manifest, error) {
	//manifest, cacheErr := newFromCache(data, ".cache")
	//if cacheErr != nil {
	var manifest *Manifest
	var delimiter = []byte("---")
	var err error
	body := append([]byte{}, data...)
	// Process front-matter, if any.
	if bytes.HasPrefix(body, delimiter) {
		parts := bytes.SplitN(body, delimiter, 3)
		if body, err = yaml.YAMLToJSON(parts[1]); err != nil {
			return nil, err
		}
		if len(parts[2]) > 0 {
			if body, err = sjson.SetBytes(body, "spec.body", parts[2]); err != nil {
				return nil, err
			}
		}
	}
	if err = json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if manifest.Meta == nil {
		manifest.Meta = &Meta{}
	}
	if manifest.Render == nil {
		manifest.Render = &Render{}
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	if manifest.Meta.HrefBase == "" {
		manifest.Meta.HrefBase = "/"
	}
	if manifest.Meta.Href == "" {
		manifest.Meta.Href = "index.html"
	}
	if manifest.Meta.Href != "" {
		parse, timeErr := strftime.New(manifest.Meta.Href)
		if timeErr != nil {
			return nil, timeErr
		}
		manifest.Meta.Href = parse.FormatString(manifest.PublishAt())
	}
	manifest.Raw = data
	//}
	manifest.Source = source
	//go saveToCache(manifest, ".cache")
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
			return nil, fmt.Errorf("%s: yaml to json failure: %w", filepath, err)
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
	go watch(len(docs), progress)
	for _, doc := range docs {
		progress <- struct{}{}
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
	go watch(len(files), progress)
	for _, path := range files {
		results, err := NewFromFile(path)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, results...)
		progress <- struct{}{}
	}
	close(progress)
	return manifests, nil
}
