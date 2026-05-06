package ifc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniversalReaderSet(t *testing.T) {
	u := NewUniversalReaderSet[string]()

	t.Run("IsUniversal", func(t *testing.T) {
		assert.True(t, u.IsUniversal())
	})

	t.Run("IsSubset", func(t *testing.T) {
		u2 := NewUniversalReaderSet[string]()
		assert.True(t, u.IsSubset(u2))

		finite := NewFiniteReaderSetFromSlice([]string{"alice", "bob"})
		assert.False(t, u.IsSubset(finite))
	})

	t.Run("Union", func(t *testing.T) {
		finite := NewFiniteReaderSetFromSlice([]string{"alice"})
		result := u.Union(finite)
		assert.True(t, result.IsUniversal())
	})

	t.Run("Intersection", func(t *testing.T) {
		finite := NewFiniteReaderSetFromSlice([]string{"alice"})
		result := u.Intersection(finite)
		assert.False(t, result.IsUniversal())
		assert.Equal(t, finite.String(), result.String())
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "UniversalReaderSet()", u.String())
	})
}

func TestFiniteReaderSet(t *testing.T) {
	t.Run("IsUniversal", func(t *testing.T) {
		f := NewFiniteReaderSetFromSlice([]string{"alice", "bob"})
		assert.False(t, f.IsUniversal())
	})

	t.Run("IsSubset", func(t *testing.T) {
		tests := []struct {
			name     string
			set      []string
			other    []string
			expected bool
		}{
			{"empty subset of any", []string{}, []string{"alice"}, true},
			{"set subset of itself", []string{"alice"}, []string{"alice"}, true},
			{"proper subset", []string{"alice"}, []string{"alice", "bob"}, true},
			{"not subset", []string{"alice", "bob"}, []string{"alice"}, false},
			{"disjoint not subset", []string{"alice"}, []string{"bob"}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := NewFiniteReaderSetFromSlice(tt.set)
				other := NewFiniteReaderSetFromSlice(tt.other)
				assert.Equal(t, tt.expected, f.IsSubset(other))
			})
		}

		t.Run("finite subset of universal", func(t *testing.T) {
			f := NewFiniteReaderSetFromSlice([]string{"alice"})
			u := NewUniversalReaderSet[string]()
			assert.True(t, f.IsSubset(u))
		})
	})

	t.Run("Union", func(t *testing.T) {
		tests := []struct {
			name     string
			set      []string
			other    []string
			expected []string
		}{
			{"empty with empty", []string{}, []string{}, []string{}},
			{"empty with non-empty", []string{}, []string{"alice"}, []string{"alice"}},
			{"disjoint sets", []string{"alice"}, []string{"bob"}, []string{"alice", "bob"}},
			{"overlapping sets", []string{"alice", "bob"}, []string{"bob", "charlie"}, []string{"alice", "bob", "charlie"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f1 := NewFiniteReaderSetFromSlice(tt.set)
				f2 := NewFiniteReaderSetFromSlice(tt.other)
				result := f1.Union(f2).(*FiniteReaderSet[string])

				expected := NewFiniteReaderSetFromSlice(tt.expected)
				assert.True(t, result.IsSubset(expected))
				assert.True(t, expected.IsSubset(result))
			})
		}

		t.Run("union with universal", func(t *testing.T) {
			f := NewFiniteReaderSetFromSlice([]string{"alice"})
			u := NewUniversalReaderSet[string]()
			result := f.Union(u)
			assert.True(t, result.IsUniversal())
		})
	})

	t.Run("Intersection", func(t *testing.T) {
		tests := []struct {
			name     string
			set      []string
			other    []string
			expected []string
		}{
			{"empty with empty", []string{}, []string{}, []string{}},
			{"empty with non-empty", []string{}, []string{"alice"}, []string{}},
			{"disjoint sets", []string{"alice"}, []string{"bob"}, []string{}},
			{"overlapping sets", []string{"alice", "bob"}, []string{"bob", "charlie"}, []string{"bob"}},
			{"identical sets", []string{"alice", "bob"}, []string{"alice", "bob"}, []string{"alice", "bob"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f1 := NewFiniteReaderSetFromSlice(tt.set)
				f2 := NewFiniteReaderSetFromSlice(tt.other)
				result := f1.Intersection(f2).(*FiniteReaderSet[string])

				expected := NewFiniteReaderSetFromSlice(tt.expected)
				assert.True(t, result.IsSubset(expected))
				assert.True(t, expected.IsSubset(result))
			})
		}

		t.Run("intersection with universal", func(t *testing.T) {
			f := NewFiniteReaderSetFromSlice([]string{"alice"})
			u := NewUniversalReaderSet[string]()
			result := f.Intersection(u)
			assert.False(t, result.IsUniversal())
			assert.Equal(t, f.String(), result.String())
		})
	})

	t.Run("String deterministic sorted", func(t *testing.T) {
		tests := []struct {
			name     string
			input    []string
			expected string
		}{
			{"empty set", []string{}, "FiniteReaderSet({})"},
			{"single element", []string{"alice"}, "FiniteReaderSet({alice})"},
			{"multiple elements", []string{"charlie", "alice", "bob"}, "FiniteReaderSet({alice, bob, charlie})"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := NewFiniteReaderSetFromSlice(tt.input)
				assert.Equal(t, tt.expected, f.String())

				f2 := NewFiniteReaderSetFromSlice(tt.input)
				assert.Equal(t, f.String(), f2.String())
			})
		}
	})
}

type unknownReaderSet struct{}

func (u *unknownReaderSet) IsUniversal() bool                                  { return false }
func (u *unknownReaderSet) IsSubset(_ ReaderSet[string]) bool                  { return false }
func (u *unknownReaderSet) Union(_ ReaderSet[string]) ReaderSet[string]        { return u }
func (u *unknownReaderSet) Intersection(_ ReaderSet[string]) ReaderSet[string] { return u }
func (u *unknownReaderSet) String() string                                     { return "unknown" }

func TestFiniteReaderSetPanicsOnUnknownType(t *testing.T) {
	unknown := &unknownReaderSet{}

	t.Run("IsSubset panics", func(t *testing.T) {
		f := NewFiniteReaderSetFromSlice([]string{"alice"})
		assert.Panics(t, func() {
			f.IsSubset(unknown)
		})
	})

	t.Run("Union panics", func(t *testing.T) {
		f := NewFiniteReaderSetFromSlice([]string{"alice"})
		assert.Panics(t, func() {
			f.Union(unknown)
		})
	})

	t.Run("Intersection panics", func(t *testing.T) {
		f := NewFiniteReaderSetFromSlice([]string{"alice"})
		assert.Panics(t, func() {
			f.Intersection(unknown)
		})
	})
}

func TestPowersetLatticeConstruction(t *testing.T) {
	universe := NewFiniteReaderSetFromSlice([]string{"alice", "bob", "charlie"})

	t.Run("valid subset", func(t *testing.T) {
		subset := NewFiniteReaderSetFromSlice([]string{"alice", "bob"})
		pl, err := NewPowersetLattice(subset, universe)
		require.NoError(t, err)
		assert.NotNil(t, pl)
	})

	t.Run("empty subset valid", func(t *testing.T) {
		subset := EmptyReaderSet[string]()
		pl, err := NewPowersetLattice(subset, universe)
		require.NoError(t, err)
		assert.NotNil(t, pl)
	})

	t.Run("universe as subset valid", func(t *testing.T) {
		pl, err := NewPowersetLattice(universe, universe)
		require.NoError(t, err)
		assert.NotNil(t, pl)
	})

	t.Run("invalid subset returns error", func(t *testing.T) {
		invalidSubset := NewFiniteReaderSetFromSlice([]string{"alice", "david"})
		pl, err := NewPowersetLattice(invalidSubset, universe)
		assert.Error(t, err)
		assert.Nil(t, pl)
	})

	t.Run("universal universe accepts any finite subset", func(t *testing.T) {
		universalUniverse := UniversalReaders[string]()
		subset := NewFiniteReaderSetFromSlice([]string{"alice", "anyone"})
		pl, err := NewPowersetLattice(subset, universalUniverse)
		require.NoError(t, err)
		assert.NotNil(t, pl)
	})
}

func TestPowersetLatticeLaws(t *testing.T) {
	universe := NewFiniteReaderSetFromSlice([]string{"alice", "bob", "charlie"})

	createPowerset := func(readers []string) *PowersetLattice[string] {
		subset := NewFiniteReaderSetFromSlice(readers)
		pl, _ := NewPowersetLattice(subset, universe)
		return pl
	}

	t.Run("Leq reflexivity", func(t *testing.T) {
		tests := [][]string{
			{},
			{"alice"},
			{"alice", "bob"},
			{"alice", "bob", "charlie"},
		}

		for _, readers := range tests {
			pl := createPowerset(readers)
			assert.True(t, pl.Leq(pl))
		}
	})

	t.Run("Join idempotency", func(t *testing.T) {
		pl := createPowerset([]string{"alice", "bob"})
		joinResult := pl.Join(pl)
		assert.True(t, pl.Leq(joinResult))
		assert.True(t, joinResult.Leq(pl))
	})

	t.Run("Meet idempotency", func(t *testing.T) {
		pl := createPowerset([]string{"alice", "bob"})
		meetResult := pl.Meet(pl)
		assert.True(t, pl.Leq(meetResult))
		assert.True(t, meetResult.Leq(pl))
	})

	t.Run("Join commutativity", func(t *testing.T) {
		pl1 := createPowerset([]string{"alice"})
		pl2 := createPowerset([]string{"bob"})

		join1 := pl1.Join(pl2)
		join2 := pl2.Join(pl1)

		assert.True(t, join1.Leq(join2))
		assert.True(t, join2.Leq(join1))
	})

	t.Run("Meet commutativity", func(t *testing.T) {
		pl1 := createPowerset([]string{"alice", "bob"})
		pl2 := createPowerset([]string{"bob", "charlie"})

		meet1 := pl1.Meet(pl2)
		meet2 := pl2.Meet(pl1)

		assert.True(t, meet1.Leq(meet2))
		assert.True(t, meet2.Leq(meet1))
	})
}

func TestPowersetLatticeMustMatchUniverse(t *testing.T) {
	universe1 := NewFiniteReaderSetFromSlice([]string{"alice", "bob"})
	universe2 := NewFiniteReaderSetFromSlice([]string{"charlie", "david"})

	t.Run("mismatched finite universes panic", func(t *testing.T) {
		pl1, _ := NewPowersetLattice(NewFiniteReaderSetFromSlice([]string{"alice"}), universe1)
		pl2, _ := NewPowersetLattice(NewFiniteReaderSetFromSlice([]string{"charlie"}), universe2)

		assert.Panics(t, func() { pl1.Leq(pl2) })
		assert.Panics(t, func() { pl1.Join(pl2) })
		assert.Panics(t, func() { pl1.Meet(pl2) })
	})

	t.Run("universal vs finite universe panic", func(t *testing.T) {
		universalUniverse := UniversalReaders[string]()
		pl1, _ := NewPowersetLattice(NewFiniteReaderSetFromSlice([]string{"alice"}), universe1)
		pl2, _ := NewPowersetLattice(NewFiniteReaderSetFromSlice([]string{"alice"}), universalUniverse)

		assert.Panics(t, func() { pl1.Leq(pl2) })
		assert.Panics(t, func() { pl1.Join(pl2) })
		assert.Panics(t, func() { pl1.Meet(pl2) })
	})

	t.Run("same universe does not panic", func(t *testing.T) {
		pl1, _ := NewPowersetLattice(NewFiniteReaderSetFromSlice([]string{"alice"}), universe1)
		pl2, _ := NewPowersetLattice(NewFiniteReaderSetFromSlice([]string{"bob"}), universe1)

		assert.NotPanics(t, func() {
			pl1.Leq(pl2)
			pl1.Join(pl2)
			pl1.Meet(pl2)
		})
	})
}

func TestReadersSecurityLabelConstructors(t *testing.T) {
	t.Run("PublicTrusted", func(t *testing.T) {
		label := PublicTrusted()
		assert.True(t, label.IsHighIntegrity())
		assert.True(t, label.IsPublicConfidentiality())
	})

	t.Run("PublicUntrusted", func(t *testing.T) {
		label := PublicUntrusted()
		assert.True(t, label.IsLowIntegrity())
		assert.True(t, label.IsPublicConfidentiality())
	})

	t.Run("PrivateTrusted", func(t *testing.T) {
		label := PrivateTrusted([]string{"alice", "bob"})
		assert.True(t, label.IsHighIntegrity())
		assert.False(t, label.IsPublicConfidentiality())
		readers := label.GetReaders()
		assert.Equal(t, []string{"alice", "bob"}, readers)
	})

	t.Run("PrivateUntrusted", func(t *testing.T) {
		label := PrivateUntrusted([]string{"alice"})
		assert.True(t, label.IsLowIntegrity())
		assert.False(t, label.IsPublicConfidentiality())
		readers := label.GetReaders()
		assert.Equal(t, []string{"alice"}, readers)
	})
}

func TestReadersSecurityLabelLeq(t *testing.T) {
	publicTrusted := PublicTrusted()
	publicUntrusted := PublicUntrusted()
	privateTrusted := PrivateTrusted([]string{"alice"})
	privateUntrusted := PrivateUntrusted([]string{"alice"})

	t.Run("public <= private in inverse lattice", func(t *testing.T) {
		assert.True(t, publicTrusted.Leq(privateTrusted))
		assert.True(t, publicUntrusted.Leq(privateUntrusted))
	})

	t.Run("private not <= public", func(t *testing.T) {
		assert.False(t, privateTrusted.Leq(publicTrusted))
		assert.False(t, privateUntrusted.Leq(publicUntrusted))
	})

	t.Run("trusted <= untrusted", func(t *testing.T) {
		assert.True(t, publicTrusted.Leq(publicUntrusted))
		assert.True(t, privateTrusted.Leq(privateUntrusted))
	})

	t.Run("untrusted not <= trusted", func(t *testing.T) {
		assert.False(t, publicUntrusted.Leq(publicTrusted))
		assert.False(t, privateUntrusted.Leq(privateTrusted))
	})

	t.Run("reflexivity", func(t *testing.T) {
		assert.True(t, publicTrusted.Leq(publicTrusted))
		assert.True(t, privateTrusted.Leq(privateTrusted))
	})
}

func TestReadersSecurityLabelJoin(t *testing.T) {
	publicTrusted := PublicTrusted()
	privateTrustedAlice := PrivateTrusted([]string{"alice"})
	privateTrustedBob := PrivateTrusted([]string{"bob"})
	publicUntrusted := PublicUntrusted()

	t.Run("public join private equals private", func(t *testing.T) {
		result := publicTrusted.Join(privateTrustedAlice)
		assert.False(t, result.IsPublicConfidentiality())
		readers := result.GetReaders()
		assert.Equal(t, []string{"alice"}, readers)
	})

	t.Run("private join public equals private", func(t *testing.T) {
		result := privateTrustedAlice.Join(publicTrusted)
		assert.False(t, result.IsPublicConfidentiality())
		readers := result.GetReaders()
		assert.Equal(t, []string{"alice"}, readers)
	})

	t.Run("alice join bob equals intersection", func(t *testing.T) {
		result := privateTrustedAlice.Join(privateTrustedBob)
		assert.False(t, result.IsPublicConfidentiality())
		readers := result.GetReaders()
		assert.Empty(t, readers)
	})

	t.Run("trusted join untrusted equals untrusted", func(t *testing.T) {
		result := publicTrusted.Join(publicUntrusted)
		assert.True(t, result.IsLowIntegrity())
	})
}

func TestReadersSecurityLabelMeet(t *testing.T) {
	publicTrusted := PublicTrusted()
	privateTrustedAlice := PrivateTrusted([]string{"alice"})
	privateTrustedBob := PrivateTrusted([]string{"bob"})
	publicUntrusted := PublicUntrusted()
	privateTrustedAliceBob := PrivateTrusted([]string{"alice", "bob"})

	t.Run("alice meet bob equals union", func(t *testing.T) {
		result := privateTrustedAlice.Meet(privateTrustedBob)
		readers := result.GetReaders()
		assert.ElementsMatch(t, []string{"alice", "bob"}, readers)
	})

	t.Run("private meet public equals public", func(t *testing.T) {
		result := privateTrustedAlice.Meet(publicTrusted)
		assert.True(t, result.IsPublicConfidentiality())
	})

	t.Run("alice meet alice-bob equals alice-bob", func(t *testing.T) {
		result := privateTrustedAlice.Meet(privateTrustedAliceBob)
		readers := result.GetReaders()
		assert.ElementsMatch(t, []string{"alice", "bob"}, readers)
	})

	t.Run("trusted meet untrusted equals trusted", func(t *testing.T) {
		result := publicTrusted.Meet(publicUntrusted)
		assert.True(t, result.IsHighIntegrity())
	})
}

func TestGetReaders(t *testing.T) {
	t.Run("public returns nil", func(t *testing.T) {
		label := PublicTrusted()
		assert.Nil(t, label.GetReaders())
	})

	t.Run("private returns sorted slice", func(t *testing.T) {
		label := PrivateTrusted([]string{"charlie", "alice", "bob"})
		readers := label.GetReaders()
		assert.Equal(t, []string{"alice", "bob", "charlie"}, readers)
	})

	t.Run("empty private returns empty slice", func(t *testing.T) {
		label := PrivateTrusted([]string{})
		readers := label.GetReaders()
		assert.NotNil(t, readers)
		assert.Empty(t, readers)
	})
}

func TestReadersSecurityLabelJSON(t *testing.T) {
	t.Run("public round-trip", func(t *testing.T) {
		original := PublicTrusted()
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored ReadersSecurityLabel
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Leq(restored))
		assert.True(t, restored.Leq(original))
		assert.True(t, restored.IsPublicConfidentiality())
		assert.True(t, restored.IsHighIntegrity())
	})

	t.Run("private round-trip", func(t *testing.T) {
		original := PrivateTrusted([]string{"alice", "bob"})
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored ReadersSecurityLabel
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Leq(restored))
		assert.True(t, restored.Leq(original))
		assert.False(t, restored.IsPublicConfidentiality())
		assert.ElementsMatch(t, []string{"alice", "bob"}, restored.GetReaders())
	})

	t.Run("untrusted round-trip", func(t *testing.T) {
		original := PrivateUntrusted([]string{"alice"})
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored ReadersSecurityLabel
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Leq(restored))
		assert.True(t, restored.Leq(original))
		assert.True(t, restored.IsLowIntegrity())
	})
}

func TestToDict(t *testing.T) {
	t.Run("public trusted", func(t *testing.T) {
		label := PublicTrusted()
		dict := label.ToDict()

		assert.Equal(t, "high", dict["integrity"])
		confidentiality := dict["confidentiality"].([]string)
		assert.Equal(t, []string{"public"}, confidentiality)
	})

	t.Run("public untrusted", func(t *testing.T) {
		label := PublicUntrusted()
		dict := label.ToDict()

		assert.Equal(t, "low", dict["integrity"])
		confidentiality := dict["confidentiality"].([]string)
		assert.Equal(t, []string{"public"}, confidentiality)
	})

	t.Run("private sorted readers", func(t *testing.T) {
		label := PrivateTrusted([]string{"charlie", "alice", "bob"})
		dict := label.ToDict()

		assert.Equal(t, "high", dict["integrity"])
		confidentiality := dict["confidentiality"].([]string)
		assert.Equal(t, []string{"alice", "bob", "charlie"}, confidentiality)
	})
}

func TestReadersSecurityLabelFromDict(t *testing.T) {
	t.Run("parse public", func(t *testing.T) {
		dict := map[string]any{
			"integrity":       "high",
			"confidentiality": []string{"public"},
		}

		label := ReadersSecurityLabelFromDict(dict)
		assert.True(t, label.IsHighIntegrity())
		assert.True(t, label.IsPublicConfidentiality())
	})

	t.Run("parse private", func(t *testing.T) {
		dict := map[string]any{
			"integrity":       "low",
			"confidentiality": []string{"alice", "bob"},
		}

		label := ReadersSecurityLabelFromDict(dict)
		assert.True(t, label.IsLowIntegrity())
		assert.False(t, label.IsPublicConfidentiality())
		readers := label.GetReaders()
		assert.ElementsMatch(t, []string{"alice", "bob"}, readers)
	})

	t.Run("parse with []any confidentiality", func(t *testing.T) {
		dict := map[string]any{
			"integrity":       "high",
			"confidentiality": []any{"alice", "bob"},
		}

		label := ReadersSecurityLabelFromDict(dict)
		readers := label.GetReaders()
		assert.ElementsMatch(t, []string{"alice", "bob"}, readers)
	})

	t.Run("defaults to high integrity", func(t *testing.T) {
		dict := map[string]any{
			"confidentiality": []string{"public"},
		}

		label := ReadersSecurityLabelFromDict(dict)
		assert.True(t, label.IsHighIntegrity())
	})

	t.Run("defaults to public confidentiality", func(t *testing.T) {
		dict := map[string]any{
			"integrity": "high",
		}

		label := ReadersSecurityLabelFromDict(dict)
		assert.True(t, label.IsPublicConfidentiality())
	})
}
