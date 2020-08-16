package cinemeta

type mediaType int

const (
	movie mediaType = iota + 1
	tvShow
)

func (mt mediaType) String() string {
	return [...]string{"movie", "TV show"}[mt-1]
}

type cinemetaResponse struct {
	Meta Meta `json:"meta"`
}

// Meta represents a movie or TV show.
type Meta struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`

	// Optional
	Genres      []string `json:"genres,omitempty"`
	Director    []string `json:"director,omitempty"`
	Cast        []string `json:"cast,omitempty"`
	Poster      string   `json:"poster,omitempty"`
	PosterShape string   `json:"posterShape,omitempty"`
	Background  string   `json:"background,omitempty"`
	Logo        string   `json:"logo,omitempty"`
	Description string   `json:"description,omitempty"`
	ReleaseInfo string   `json:"releaseInfo,omitempty"` // A.k.a. *year*. E.g. "2000" for movies and "2000-2014" or "2000-" for TV shows
	IMDbRating  string   `json:"imdbRating,omitempty"`
	Released    string   `json:"released,omitempty"` // ISO 8601, e.g. "2010-12-06T05:00:00.000Z"
	Runtime     string   `json:"runtime,omitempty"`
	Language    string   `json:"language,omitempty"`
	Country     string   `json:"country,omitempty"`
	Awards      string   `json:"awards,omitempty"`
	Website     string   `json:"website,omitempty"`
}
