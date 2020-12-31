package stremio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestClone(t *testing.T) {
	// Test empty struct to make sure empty slices are nil and not slices with 0 elements.
	m := Manifest{}
	require.Equal(t, m, m.clone())

	// Fill every field to ensure initial equality after the clone.
	m = Manifest{
		ID:          "com.example.some-addon",
		Name:        "Some addon",
		Description: "Some addon",
		Version:     "0.1.0",

		ResourceItems: []ResourceItem{
			{
				Name:  "catalog",
				Types: []string{"movie"},

				IDprefixes: []string{"tt"},
			},
		},

		Types: []string{"movie"},
		Catalogs: []CatalogItem{
			{
				Type: "movie",
				ID:   "some-catalog",
				Name: "Some catalog",

				Extra: []ExtraItem{
					{
						Name: "Some extra",

						IsRequired:   true,
						Options:      []string{"foo"},
						OptionsLimit: 123,
					},
				},
			},
		},

		IDprefixes:   []string{"tt"},
		Background:   "https://example.com/background.jpg",
		Logo:         "https://example.com/logo.png",
		ContactEmail: "mail@example.com",
		BehaviorHints: BehaviorHints{
			Adult:                 true,
			P2P:                   true,
			Configurable:          true,
			ConfigurationRequired: true,
		},
	}
	require.Equal(t, m, m.clone())

	// Create a list of test scenarios, where each one alters a single field.
	// The only fields we care about here are non-simple types, because simple types are deep-copied by default.
	tests := []struct {
		name string
		f    func(m *Manifest)
	}{
		{
			name: "ID",
			f:    func(m *Manifest) { m.ID = "changed" },
		},
		{
			name: "ResourceItems.Name",
			f:    func(m *Manifest) { m.ResourceItems[0].Name = "changed" },
		},
		{
			name: "ResourceItems.Types",
			f:    func(m *Manifest) { m.ResourceItems[0].Types[0] = "changed" },
		},
		{
			name: "ResourceItems.IDprefixes",
			f:    func(m *Manifest) { m.ResourceItems[0].IDprefixes[0] = "changed" },
		},
		{
			name: "Types",
			f:    func(m *Manifest) { m.Types[0] = "changed" },
		},
		{
			name: "Catalogs.Type",
			f:    func(m *Manifest) { m.Catalogs[0].Type = "changed" },
		},
		{
			name: "Catalogs.Extra.Name",
			f:    func(m *Manifest) { m.Catalogs[0].Extra[0].Name = "changed" },
		},
		{
			name: "Catalogs.Extra.Options",
			f:    func(m *Manifest) { m.Catalogs[0].Extra[0].Options[0] = "changed" },
		},
		{
			name: "IDprefixes",
			f:    func(m *Manifest) { m.IDprefixes[0] = "changed" },
		},
		{
			name: "BehaviorHints",
			f:    func(m *Manifest) { m.BehaviorHints.Adult = false },
		},
	}

	// For each scenario, clone the original manifest, then run the scenario func, then compare.
	// We expect UNequality for each.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m2 := m.clone()
			test.f(&m2)
			require.NotEqual(t, m, m2)
			// Each time the NotEqual succeeds it means that m is not altered and thus we can safely go to the next scenario without fearing that the next scenario might only succeed because a previous unequality is still around
		})
	}
}
