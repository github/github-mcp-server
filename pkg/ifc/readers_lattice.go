// readers_lattice.go extends the basic lattice structures with reader-based confidentiality.
//
// It provides:
//   - Powerset lattice with support for both finite sets and universal sets
//   - ReadersSecurityLabel combining integrity with reader-based confidentiality
//
// The confidentiality dimension uses an inverse powerset lattice over reader sets,
// where fewer readers means higher confidentiality (more restrictive).
// UniversalReaderSet represents "public" data readable by everyone.
//
// Example usage:
//
//	// Create a label readable by everyone (public)
//	universe := UniversalReaders[string]()
//	label, _ := NewPowersetLattice(universe, universe)
//
//	// Create a label for specific readers only
//	finiteUniverse := NewFiniteReaderSetFromSlice([]string{"alice", "bob", "charlie"})
//	privateReaders := NewFiniteReaderSetFromSlice([]string{"alice"})
//	privateLabel, _ := NewPowersetLattice(privateReaders, finiteUniverse)

package ifc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ReaderSet represents either a finite set of readers or the universal set of all readers.
// This allows representing "all possible readers" without enumerating them.
type ReaderSet[T comparable] interface {
	IsUniversal() bool
	IsSubset(other ReaderSet[T]) bool
	Union(other ReaderSet[T]) ReaderSet[T]
	Intersection(other ReaderSet[T]) ReaderSet[T]
	fmt.Stringer
}

// UniversalReaderSet represents the universe of all possible readers.
// Any finite set is considered a subset of the universal set.
type UniversalReaderSet[T comparable] struct{}

func NewUniversalReaderSet[T comparable]() *UniversalReaderSet[T] {
	return &UniversalReaderSet[T]{}
}

func (u *UniversalReaderSet[T]) IsUniversal() bool {
	return true
}

func (u *UniversalReaderSet[T]) IsSubset(other ReaderSet[T]) bool {
	// Universal set is only a subset of itself
	return other.IsUniversal()
}

func (u *UniversalReaderSet[T]) Union(_ ReaderSet[T]) ReaderSet[T] {
	// Union with universal set is always the universal set
	return u
}

func (u *UniversalReaderSet[T]) Intersection(other ReaderSet[T]) ReaderSet[T] {
	// Intersection with universal set returns the other set unchanged
	return other
}

func (u *UniversalReaderSet[T]) String() string {
	return "UniversalReaderSet()"
}

// FiniteReaderSet represents a finite set of readers.
type FiniteReaderSet[T comparable] struct {
	members map[T]struct{}
}

func NewFiniteReaderSet[T comparable](members map[T]struct{}) *FiniteReaderSet[T] {
	return &FiniteReaderSet[T]{
		members: copySet(members),
	}
}

func (f *FiniteReaderSet[T]) IsUniversal() bool {
	return false
}

func (f *FiniteReaderSet[T]) IsSubset(other ReaderSet[T]) bool {
	if other.IsUniversal() {
		return true
	}
	otherFinite, ok := other.(*FiniteReaderSet[T])
	if !ok {
		panic(fmt.Sprintf("unsupported ReaderSet implementation: %T", other))
	}
	for member := range f.members {
		if _, exists := otherFinite.members[member]; !exists {
			return false
		}
	}
	return true
}

func (f *FiniteReaderSet[T]) Union(other ReaderSet[T]) ReaderSet[T] {
	if other.IsUniversal() {
		return other
	}
	otherFinite, ok := other.(*FiniteReaderSet[T])
	if !ok {
		panic(fmt.Sprintf("unsupported ReaderSet implementation: %T", other))
	}
	union := make(map[T]struct{}, len(f.members)+len(otherFinite.members))
	for member := range f.members {
		union[member] = struct{}{}
	}
	for member := range otherFinite.members {
		union[member] = struct{}{}
	}
	return NewFiniteReaderSet(union)
}

func (f *FiniteReaderSet[T]) Intersection(other ReaderSet[T]) ReaderSet[T] {
	if other.IsUniversal() {
		return f
	}
	otherFinite, ok := other.(*FiniteReaderSet[T])
	if !ok {
		panic(fmt.Sprintf("unsupported ReaderSet implementation: %T", other))
	}
	intersection := make(map[T]struct{})
	for member := range f.members {
		if _, exists := otherFinite.members[member]; exists {
			intersection[member] = struct{}{}
		}
	}
	return NewFiniteReaderSet(intersection)
}

func (f *FiniteReaderSet[T]) String() string {
	if len(f.members) == 0 {
		return "FiniteReaderSet({})"
	}
	strs := make([]string, 0, len(f.members))
	for member := range f.members {
		strs = append(strs, fmt.Sprintf("%v", member))
	}
	sort.Strings(strs)
	var b strings.Builder
	b.WriteString("FiniteReaderSet({")
	for i, s := range strs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(s)
	}
	b.WriteString("})")
	return b.String()
}

// PowersetLattice is a powerset lattice that can represent either a finite set or the universal set.
// subset and universe are represented using the ReaderSet interface.
// T must be comparable to be used as a map key.
type PowersetLattice[T comparable] struct {
	subset   ReaderSet[T]
	universe ReaderSet[T]
}

// NewPowersetLattice constructs a PowersetLattice, checking that
// subset ⊆ universe.
func NewPowersetLattice[T comparable](subset, universe ReaderSet[T]) (*PowersetLattice[T], error) {
	if !subset.IsSubset(universe) {
		return nil, fmt.Errorf("subset must be within the universe")
	}
	return &PowersetLattice[T]{
		subset:   subset,
		universe: universe,
	}, nil
}

// NewPowersetLatticeUnchecked constructs a PowersetLattice without validation.
// Use this when you know the subset is valid.
func NewPowersetLatticeUnchecked[T comparable](subset, universe ReaderSet[T]) *PowersetLattice[T] {
	return &PowersetLattice[T]{
		subset:   subset,
		universe: universe,
	}
}

// helper to copy sets so callers don't mutate internals.
func copySet[T comparable](in map[T]struct{}) map[T]struct{} {
	out := make(map[T]struct{}, len(in))
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}

func (p *PowersetLattice[T]) Leq(other *PowersetLattice[T]) bool {
	p.mustMatchUniverse(other)
	return p.subset.IsSubset(other.subset)
}

func (p *PowersetLattice[T]) Join(other *PowersetLattice[T]) *PowersetLattice[T] {
	p.mustMatchUniverse(other)
	return &PowersetLattice[T]{
		subset:   p.subset.Union(other.subset),
		universe: p.universe,
	}
}

func (p *PowersetLattice[T]) Meet(other *PowersetLattice[T]) *PowersetLattice[T] {
	p.mustMatchUniverse(other)
	return &PowersetLattice[T]{
		subset:   p.subset.Intersection(other.subset),
		universe: p.universe,
	}
}

func (p *PowersetLattice[T]) mustMatchUniverse(other *PowersetLattice[T]) {
	pUniv := p.universe.IsUniversal()
	oUniv := other.universe.IsUniversal()
	if pUniv != oUniv {
		panic(fmt.Sprintf("universe mismatch: %s vs %s", p.universe, other.universe))
	}
	if pUniv {
		return
	}
	pFinite, pOK := p.universe.(*FiniteReaderSet[T])
	oFinite, oOK := other.universe.(*FiniteReaderSet[T])
	if !pOK || !oOK {
		panic(fmt.Sprintf("universe mismatch: %T vs %T", p.universe, other.universe))
	}
	if !pFinite.IsSubset(oFinite) || !oFinite.IsSubset(pFinite) {
		panic(fmt.Sprintf("universe mismatch: %s vs %s", p.universe, other.universe))
	}
}

func (p *PowersetLattice[T]) String() string {
	return fmt.Sprintf("Powerset(%s)", p.subset.String())
}

// Bottom returns the bottom element (empty subset).
func BottomPowerset[T comparable](universe ReaderSet[T]) *PowersetLattice[T] {
	return &PowersetLattice[T]{
		subset:   NewFiniteReaderSet[T](make(map[T]struct{})),
		universe: universe,
	}
}

// Top returns the top element (the full universe).
func TopPowerset[T comparable](universe ReaderSet[T]) *PowersetLattice[T] {
	return &PowersetLattice[T]{
		subset:   universe,
		universe: universe,
	}
}

// Satisfy Lattice[*PowersetLattice[T]].
var _ Lattice[*PowersetLattice[int]] = (*PowersetLattice[int])(nil)

// NewFiniteReaderSetFromSlice creates a FiniteReaderSet from a slice of elements.
func NewFiniteReaderSetFromSlice[T comparable](elements []T) *FiniteReaderSet[T] {
	members := make(map[T]struct{}, len(elements))
	for _, elem := range elements {
		members[elem] = struct{}{}
	}
	return NewFiniteReaderSet(members)
}

// EmptyReaderSet creates an empty finite reader set.
func EmptyReaderSet[T comparable]() *FiniteReaderSet[T] {
	return NewFiniteReaderSet[T](make(map[T]struct{}))
}

// UniversalReaders creates a universal reader set (all possible readers).
func UniversalReaders[T comparable]() *UniversalReaderSet[T] {
	return NewUniversalReaderSet[T]()
}

// ReadersSecurityLabel represents an Information Flow Control label with:
// - IntegrityLabel: TRUSTED ⊑ UNTRUSTED
// - InverseLattice[PowersetLattice[string]]: For confidentiality using readers
//
// This matches the Python implementation with proper lattice operations where:
// - public ⊔ Alice = Alice (more restrictive wins)
// - trusted ⊔ untrusted = untrusted (lower integrity wins)
type ReadersSecurityLabel struct {
	Integrity       IntegrityLabel
	Confidentiality InverseLattice[*PowersetLattice[string]]
}

// Leq returns true if self <= other in the lattice.
func (l ReadersSecurityLabel) Leq(other ReadersSecurityLabel) bool {
	return l.Integrity.Leq(other.Integrity) &&
		l.Confidentiality.Leq(other.Confidentiality)
}

// Join returns the least upper bound of self and other.
// For integrity: TRUSTED ⊔ UNTRUSTED = UNTRUSTED
// For confidentiality: public ⊔ Alice = Alice (intersection of readers = more restrictive)
func (l ReadersSecurityLabel) Join(other ReadersSecurityLabel) ReadersSecurityLabel {
	return ReadersSecurityLabel{
		Integrity:       l.Integrity.Join(other.Integrity),
		Confidentiality: l.Confidentiality.Join(other.Confidentiality),
	}
}

// Meet returns the greatest lower bound of self and other.
func (l ReadersSecurityLabel) Meet(other ReadersSecurityLabel) ReadersSecurityLabel {
	return ReadersSecurityLabel{
		Integrity:       l.Integrity.Meet(other.Integrity),
		Confidentiality: l.Confidentiality.Meet(other.Confidentiality),
	}
}

// IsLowIntegrity returns true if this label has untrusted integrity.
func (l ReadersSecurityLabel) IsLowIntegrity() bool {
	return l.Integrity.Level == IntegrityUntrusted
}

// IsHighIntegrity returns true if this label has trusted integrity.
func (l ReadersSecurityLabel) IsHighIntegrity() bool {
	return l.Integrity.Level == IntegrityTrusted
}

// IsPublicConfidentiality returns true if the confidentiality is public (universal readers).
func (l ReadersSecurityLabel) IsPublicConfidentiality() bool {
	return l.Confidentiality.Inner.subset.IsUniversal()
}

// GetReaders returns the set of readers for this label.
// Returns nil if the readers are universal (public).
func (l ReadersSecurityLabel) GetReaders() []string {
	if l.IsPublicConfidentiality() {
		return nil
	}

	finiteSet, ok := l.Confidentiality.Inner.subset.(*FiniteReaderSet[string])
	if !ok {
		return nil
	}

	readers := make([]string, 0, len(finiteSet.members))
	for reader := range finiteSet.members {
		readers = append(readers, reader)
	}
	sort.Strings(readers)
	return readers
}

// ReaderSetFromList creates a ReaderSet from a list of readers.
func ReaderSetFromList(readers []string) ReaderSet[string] {
	if len(readers) == 0 {
		return NewFiniteReaderSet[string](make(map[string]struct{}))
	}
	if len(readers) == 1 && readers[0] == "public" {
		return NewUniversalReaderSet[string]()
	}
	members := make(map[string]struct{}, len(readers))
	for _, r := range readers {
		members[r] = struct{}{}
	}
	return NewFiniteReaderSet(members)
}

func ConfidentialityLabelFromReaderSet(readers ReaderSet[string]) InverseLattice[*PowersetLattice[string]] {
	universe := UniversalReaders[string]()
	return InverseLattice[*PowersetLattice[string]]{
		Inner: NewPowersetLatticeUnchecked(readers, universe),
	}
}

// String returns a human-readable representation of the label.
func (l ReadersSecurityLabel) String() string {
	integrityStr := "trusted"
	if l.IsLowIntegrity() {
		integrityStr = "untrusted"
	}

	confStr := "public"
	if !l.IsPublicConfidentiality() {
		readers := l.GetReaders()
		confStr = fmt.Sprintf("{%v}", readers)
	}

	return fmt.Sprintf("ReadersSecurityLabel(%s, %s)", integrityStr, confStr)
}

// ToDict converts the label to a dictionary format for backward compatibility and serialization.
func (l ReadersSecurityLabel) ToDict() map[string]any {
	integrityStr := "high"
	if l.IsLowIntegrity() {
		integrityStr = "low"
	}

	confidentiality := []string{"public"}
	if !l.IsPublicConfidentiality() {
		confidentiality = l.GetReaders()
		if confidentiality == nil {
			confidentiality = []string{}
		}
	}

	return map[string]any{
		"integrity":       integrityStr,
		"confidentiality": confidentiality,
	}
}

// MarshalJSON implements json.Marshaler.
func (l ReadersSecurityLabel) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.ToDict())
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *ReadersSecurityLabel) UnmarshalJSON(data []byte) error {
	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}

	*l = ReadersSecurityLabelFromDict(dict)
	return nil
}

// ReadersSecurityLabelFromDict creates a ReadersSecurityLabel from a dictionary format.
func ReadersSecurityLabelFromDict(dict map[string]any) ReadersSecurityLabel {
	// Parse integrity
	integrityStr := "high"
	if i, ok := dict["integrity"].(string); ok {
		integrityStr = i
	}

	var integrity IntegrityLabel
	if integrityStr == "low" {
		integrity = Untrusted()
	} else {
		integrity = Trusted()
	}

	// Parse confidentiality
	confList := []string{"public"}
	if c, ok := dict["confidentiality"].([]any); ok {
		confList = make([]string, len(c))
		for i, v := range c {
			if s, ok := v.(string); ok {
				confList[i] = s
			}
		}
	} else if c, ok := dict["confidentiality"].([]string); ok {
		confList = c
	}

	// Check if it's public
	isPublic := len(confList) == 1 && confList[0] == "public"

	var confidentiality InverseLattice[*PowersetLattice[string]]
	universe := UniversalReaders[string]()

	if isPublic {
		// Public means universal readers
		confidentiality = InverseLattice[*PowersetLattice[string]]{
			Inner: TopPowerset(universe),
		}
	} else {
		// Specific readers
		readers := NewFiniteReaderSetFromSlice(confList)
		confidentiality = InverseLattice[*PowersetLattice[string]]{
			Inner: NewPowersetLatticeUnchecked(readers, universe),
		}
	}

	return ReadersSecurityLabel{
		Integrity:       integrity,
		Confidentiality: confidentiality,
	}
}

// PublicTrusted creates a public trusted label (most permissive).
func PublicTrusted() ReadersSecurityLabel {
	universe := UniversalReaders[string]()
	return ReadersSecurityLabel{
		Integrity: Trusted(),
		Confidentiality: InverseLattice[*PowersetLattice[string]]{
			Inner: TopPowerset(universe),
		},
	}
}

// PublicUntrusted creates a public untrusted label.
func PublicUntrusted() ReadersSecurityLabel {
	universe := UniversalReaders[string]()
	return ReadersSecurityLabel{
		Integrity: Untrusted(),
		Confidentiality: InverseLattice[*PowersetLattice[string]]{
			Inner: TopPowerset(universe),
		},
	}
}

// PrivateTrusted creates a private trusted label for specific readers.
func PrivateTrusted(readers []string) ReadersSecurityLabel {
	universe := UniversalReaders[string]()
	readerSet := NewFiniteReaderSetFromSlice(readers)
	return ReadersSecurityLabel{
		Integrity: Trusted(),
		Confidentiality: InverseLattice[*PowersetLattice[string]]{
			Inner: NewPowersetLatticeUnchecked(readerSet, universe),
		},
	}
}

// PrivateUntrusted creates a private untrusted label for specific readers.
func PrivateUntrusted(readers []string) ReadersSecurityLabel {
	universe := UniversalReaders[string]()
	readerSet := NewFiniteReaderSetFromSlice(readers)
	return ReadersSecurityLabel{
		Integrity: Untrusted(),
		Confidentiality: InverseLattice[*PowersetLattice[string]]{
			Inner: NewPowersetLatticeUnchecked(readerSet, universe),
		},
	}
}

// ReadersLabel builds a confidentiality label from a readers set.
func ReadersLabel(readers ReaderSet[string]) InverseLattice[*PowersetLattice[string]] {
	universe := UniversalReaders[string]()
	return InverseLattice[*PowersetLattice[string]]{
		Inner: NewPowersetLatticeUnchecked(readers, universe),
	}
}

// Predefined labels for common cases
var (
	ReadersSecurityLabelPublicTrusted   = PublicTrusted()
	ReadersSecurityLabelPublicUntrusted = PublicUntrusted()
)
