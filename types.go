package stremio

// Manifest describes the capabilities of the addon.
// See https://github.com/Stremio/stremio-addon-sdk/blob/f6f1f2a8b627b9d4f2c62b003b251d98adadbebe/docs/api/responses/manifest.md
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`

	// One of the following is required
	// Note: Can only have one in code because of how Go (de-)serialization works
	//Resources     []string       `json:"resources,omitempty"`
	ResourceItems []ResourceItem `json:"resources,omitempty"`

	Types    []string      `json:"types"` // Stremio supports "movie", "series", "channel" and "tv"
	Catalogs []CatalogItem `json:"catalogs"`

	// Optional
	IDprefixes    []string      `json:"idPrefixes,omitempty"`
	Background    string        `json:"background,omitempty"` // URL
	Logo          string        `json:"logo,omitempty"`       // URL
	ContactEmail  string        `json:"contactEmail,omitempty"`
	BehaviorHints BehaviorHints `json:"behaviorHints,omitempty"`
}

// clone returns a deep copy of m.
// We're not using one of the deep copy libraries because only few are maintained and even they have issues.
func (m Manifest) clone() Manifest {
	var resourceItems []ResourceItem
	if m.ResourceItems != nil {
		resourceItems = make([]ResourceItem, len(m.ResourceItems))
		for i, resourceItem := range m.ResourceItems {
			resourceItems[i] = resourceItem.clone()
		}
	}

	var types []string
	if m.Types != nil {
		types = make([]string, len(m.Types))
		for i, t := range m.Types {
			types[i] = t
		}
	}

	var catalogs []CatalogItem
	if m.Catalogs != nil {
		catalogs = make([]CatalogItem, len(m.Catalogs))
		for i, catalog := range m.Catalogs {
			catalogs[i] = catalog.clone()
		}
	}

	var idPrefixes []string
	if m.IDprefixes != nil {
		idPrefixes = make([]string, len(m.IDprefixes))
		for i, idPrefix := range m.IDprefixes {
			idPrefixes[i] = idPrefix
		}
	}

	return Manifest{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Version:     m.Version,

		ResourceItems: resourceItems,

		Types:    types,
		Catalogs: catalogs,

		IDprefixes:    idPrefixes,
		Background:    m.Background,
		Logo:          m.Logo,
		ContactEmail:  m.ContactEmail,
		BehaviorHints: m.BehaviorHints,
	}
}

type ResourceItem struct {
	Name  string   `json:"name"`
	Types []string `json:"types"` // Stremio supports "movie", "series", "channel" and "tv"

	// Optional
	IDprefixes []string `json:"idPrefixes,omitempty"`
}

func (ri ResourceItem) clone() ResourceItem {
	var types []string
	if ri.Types != nil {
		types = make([]string, len(ri.Types))
		for i, t := range ri.Types {
			types[i] = t
		}
	}

	var idPrefixes []string
	if ri.IDprefixes != nil {
		idPrefixes = make([]string, len(ri.IDprefixes))
		for i, idPrefix := range ri.IDprefixes {
			idPrefixes[i] = idPrefix
		}
	}

	return ResourceItem{
		Name:  ri.Name,
		Types: types,

		IDprefixes: idPrefixes,
	}
}

type BehaviorHints struct {
	// Note: Must include `omitempty`, otherwise it will be included if this struct is used in another one, even if the field of the containing struct is marked as `omitempty`
	Adult        bool `json:"adult,omitempty"`
	P2P          bool `json:"p2p,omitempty"`
	Configurable bool `json:"configurable,omitempty"`
	// If you set this to true, it will be true for the "/manifest.json" endpoint, but false for the "/:userData/manifest.json" endpoint, because otherwise Stremio won't show the "Install" button in its UI.
	ConfigurationRequired bool `json:"configurationRequired,omitempty"`
}

// CatalogItem represents a catalog.
type CatalogItem struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`

	// Optional
	Extra []ExtraItem `json:"extra,omitempty"`
}

func (ci CatalogItem) clone() CatalogItem {
	var extras []ExtraItem
	if ci.Extra != nil {
		extras = make([]ExtraItem, len(ci.Extra))
		for i, extra := range ci.Extra {
			extras[i] = extra.clone()
		}
	}

	return CatalogItem{
		Type: ci.Type,
		ID:   ci.ID,
		Name: ci.Name,

		Extra: extras,
	}
}

type ExtraItem struct {
	Name string `json:"name"`

	// Optional
	IsRequired   bool     `json:"isRequired,omitempty"`
	Options      []string `json:"options,omitempty"`
	OptionsLimit int      `json:"optionsLimit,omitempty"`
}

func (ei ExtraItem) clone() ExtraItem {
	var options []string
	if ei.Options != nil {
		options = make([]string, len(ei.Options))
		for i, option := range ei.Options {
			options[i] = option
		}
	}

	return ExtraItem{
		Name: ei.Name,

		IsRequired:   ei.IsRequired,
		Options:      options,
		OptionsLimit: ei.OptionsLimit,
	}
}

// MetaPreviewItem represents a meta preview item and is meant to be used within catalog responses.
// See https://github.com/Stremio/stremio-addon-sdk/blob/f6f1f2a8b627b9d4f2c62b003b251d98adadbebe/docs/api/responses/meta.md#meta-preview-object
type MetaPreviewItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Poster string `json:"poster"` // URL

	// Optional
	PosterShape string `json:"posterShape,omitempty"`

	// Optional, used for the "Discover" page sidebar
	Genres      []string       `json:"genres,omitempty"`   // Will be replaced by Links at some point
	Director    []string       `json:"director,omitempty"` // Will be replaced by Links at some point
	Cast        []string       `json:"cast,omitempty"`     // Will be replaced by Links at some point
	Links       []MetaLinkItem `json:"links,omitempty"`    // For genres, director, cast and potentially more. Not fully supported by Stremio yet!
	IMDbRating  string         `json:"imdbRating,omitempty"`
	ReleaseInfo string         `json:"releaseInfo,omitempty"` // E.g. "2000" for movies and "2000-2014" or "2000-" for TV shows
	Description string         `json:"description,omitempty"`
}

// MetaItem represents a meta item and is meant to be used when info for a specific item was requested.
// See https://github.com/Stremio/stremio-addon-sdk/blob/f6f1f2a8b627b9d4f2c62b003b251d98adadbebe/docs/api/responses/meta.md
type MetaItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`

	// Optional
	Genres      []string       `json:"genres,omitempty"`   // Will be replaced by Links at some point
	Director    []string       `json:"director,omitempty"` // Will be replaced by Links at some point
	Cast        []string       `json:"cast,omitempty"`     // Will be replaced by Links at some point
	Links       []MetaLinkItem `json:"links,omitempty"`    // For genres, director, cast and potentially more. Not fully supported by Stremio yet!
	Poster      string         `json:"poster,omitempty"`   // URL
	PosterShape string         `json:"posterShape,omitempty"`
	Background  string         `json:"background,omitempty"` // URL
	Logo        string         `json:"logo,omitempty"`       // URL
	Description string         `json:"description,omitempty"`
	ReleaseInfo string         `json:"releaseInfo,omitempty"` // E.g. "2000" for movies and "2000-2014" or "2000-" for TV shows
	IMDbRating  string         `json:"imdbRating,omitempty"`
	Released    string         `json:"released,omitempty"` // Must be ISO 8601, e.g. "2010-12-06T05:00:00.000Z"
	Videos      []VideoItem    `json:"videos,omitempty"`
	Runtime     string         `json:"runtime,omitempty"`
	Language    string         `json:"language,omitempty"`
	Country     string         `json:"country,omitempty"`
	Awards      string         `json:"awards,omitempty"`
	Website     string         `json:"website,omitempty"` // URL

	// TODO: behaviorHints
}

// MetaLinkItem links to a page within Stremio.
// It will at some point replace the usage of `genres`, `director` and `cast`.
// Note: It's not fully supported by Stremio yet (not fully on PC and not at all on Android)!
type MetaLinkItem struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	URL      string `json:"url"` //  // URL. Can be "Meta Links" (see https://github.com/Stremio/stremio-addon-sdk/blob/f6f1f2a8b627b9d4f2c62b003b251d98adadbebe/docs/api/responses/meta.links.md)
}

type VideoItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Released string `json:"released"` // Must be ISO 8601, e.g. "2010-12-06T05:00:00.000Z"

	// Optional
	Thumbnail string       `json:"thumbnail,omitempty"` // URL
	Streams   []StreamItem `json:"streams,omitempty"`
	Available bool         `json:"available,omitempty"`
	Episode   string       `json:"episode,omitempty"`
	Season    string       `json:"season,omitempty"`
	Trailer   string       `json:"trailer,omitempty"` // Youtube ID
	Overview  string       `json:"overview,omitempty"`
}

// StreamItem represents a stream for a MetaItem.
// See https://github.com/Stremio/stremio-addon-sdk/blob/f6f1f2a8b627b9d4f2c62b003b251d98adadbebe/docs/api/responses/stream.md
type StreamItem struct {
	// One of the following is required
	URL         string `json:"url,omitempty"` // URL
	YoutubeID   string `json:"ytId,omitempty"`
	InfoHash    string `json:"infoHash,omitempty"`
	ExternalURL string `json:"externalUrl,omitempty"` // URL

	// Optional
	Title     string `json:"title,omitempty"`   // Usually used for stream quality
	Name     	string `json:"name,omitempty"`   // Usually used for stream quality
	FileIndex uint8  `json:"fileIdx,omitempty"` // Only when using InfoHash

	// TODO: subtitles
	// TODO: behaviorHints
}
