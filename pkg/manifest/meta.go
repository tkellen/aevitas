package manifest

// Meta provides details about a resource.
type Meta struct {
	// File is a temporary hack to make this system work with memorybox.
	File string
	// Live controls if a manifest is collected during production builds.
	Live bool
	// An optional prefix for this page / child pages.
	TitleBase string
	// A title for the resource.
	Title string
	// A description for the resource.
	Description string
	// An optional prefix for the resource this points to.
	HrefBase string
	// A URL for the manifest, defaults to index.html if not specified.
	Href string
	// PublishAt controls if a manifest is collected during production builds.
	// If present, current date/time must be greater than the machine that runs
	// the build. It also provides the basis for ordering manifests.
	PublishAt *PublishAt
}

// PublishAt describes in a granular fashion when a given manifest should be
// published. This deconstructed form is meant to ease filtering and relating to
// manifests by portions of their publication date from within manifest files.
type PublishAt struct {
	Year    int
	Month   int
	Day     int
	Hours   int
	Minutes int
	Seconds int
}
