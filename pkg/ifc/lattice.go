// Package ifc implements Information Flow Control (IFC) lattices and security labels.
//
// This package provides the fundamental lattice structures used for IFC:
//   - Confidentiality lattice (LOW, HIGH)
//   - Integrity lattice (TRUSTED, UNTRUSTED)
package ifc

import "fmt"

type Lattice[T any] interface {
	Leq(other T) bool // self <= other
	Join(other T) T   // least upper bound
	Meet(other T) T   // greatest lower bound
	fmt.Stringer      // String() string
}

type ConfidentialityLevel int

const (
	ConfidentialityLow ConfidentialityLevel = iota
	ConfidentialityHigh
)

func (l ConfidentialityLevel) String() string {
	switch l {
	case ConfidentialityLow:
		return "LOW"
	case ConfidentialityHigh:
		return "HIGH"
	default:
		return fmt.Sprintf("ConfidentialityLevel(%d)", int(l))
	}
}

type ConfidentialityLabel struct {
	Level ConfidentialityLevel
}

func LowConfidentiality() ConfidentialityLabel {
	return ConfidentialityLabel{Level: ConfidentialityLow}
}

func HighConfidentiality() ConfidentialityLabel {
	return ConfidentialityLabel{Level: ConfidentialityHigh}
}

type SecurityLabel struct {
	ProductLabel[ConfidentialityLabel, IntegrityLabel]
}

func NewSecurityLabel(c ConfidentialityLabel, i IntegrityLabel) SecurityLabel {
	return SecurityLabel{
		ProductLabel: ProductLabel[ConfidentialityLabel, IntegrityLabel]{
			Left:  c,
			Right: i,
		},
	}
}

func (s SecurityLabel) Leq(other SecurityLabel) bool {
	return s.ProductLabel.Leq(other.ProductLabel)
}

func (s SecurityLabel) Join(other SecurityLabel) SecurityLabel {
	return SecurityLabel{
		ProductLabel: s.ProductLabel.Join(other.ProductLabel),
	}
}

func (s SecurityLabel) Meet(other SecurityLabel) SecurityLabel {
	return SecurityLabel{
		ProductLabel: s.ProductLabel.Meet(other.ProductLabel),
	}
}

func (s SecurityLabel) String() string {
	return s.ProductLabel.String()
}

var _ Lattice[SecurityLabel] = SecurityLabel{}

var LabelHighConfidentialityTrusted = NewSecurityLabel(HighConfidentiality(), Trusted())
var LabelPublicTrusted = NewSecurityLabel(LowConfidentiality(), Trusted())
var LabelUserUntrusted = NewSecurityLabel(HighConfidentiality(), Untrusted())
var LabelPublicUntrusted = NewSecurityLabel(LowConfidentiality(), Untrusted())

func (c ConfidentialityLabel) Leq(other ConfidentialityLabel) bool {
	return int(c.Level) <= int(other.Level)
}

func (c ConfidentialityLabel) Join(other ConfidentialityLabel) ConfidentialityLabel {
	if c.Leq(other) {
		return other
	}
	return c
}

func (c ConfidentialityLabel) Meet(other ConfidentialityLabel) ConfidentialityLabel {
	if c.Leq(other) {
		return c
	}
	return other
}

func (c ConfidentialityLabel) String() string {
	return c.Level.String()
}

var _ Lattice[ConfidentialityLabel] = ConfidentialityLabel{}

type IntegrityLevel int

const (
	IntegrityTrusted IntegrityLevel = iota
	IntegrityUntrusted
)

func (l IntegrityLevel) String() string {
	switch l {
	case IntegrityTrusted:
		return "TRUSTED"
	case IntegrityUntrusted:
		return "UNTRUSTED"
	default:
		return fmt.Sprintf("IntegrityLevel(%d)", int(l))
	}
}

type IntegrityLabel struct {
	Level IntegrityLevel
}

// Trusted: content originating from the user, from trusted collaborators, or system prompts.
func Trusted() IntegrityLabel {
	return IntegrityLabel{Level: IntegrityTrusted}
}

// Untrusted: content from untrusted users (e.g., no push access), or from external/public sources.
func Untrusted() IntegrityLabel {
	return IntegrityLabel{Level: IntegrityUntrusted}
}

func (i IntegrityLabel) Leq(other IntegrityLabel) bool {
	return int(i.Level) <= int(other.Level)
}

func (i IntegrityLabel) Join(other IntegrityLabel) IntegrityLabel {
	if i.Leq(other) {
		return other
	}
	return i
}

func (i IntegrityLabel) Meet(other IntegrityLabel) IntegrityLabel {
	if i.Leq(other) {
		return i
	}
	return other
}

func (i IntegrityLabel) String() string {
	return i.Level.String()
}

var _ Lattice[IntegrityLabel] = IntegrityLabel{}

// ProductLabel is a product lattice of two lattices L1 × L2.
type ProductLabel[L1 Lattice[L1], L2 Lattice[L2]] struct {
	Left  L1
	Right L2
}

func (p ProductLabel[L1, L2]) Leq(other ProductLabel[L1, L2]) bool {
	return p.Left.Leq(other.Left) && p.Right.Leq(other.Right)
}

func (p ProductLabel[L1, L2]) Join(other ProductLabel[L1, L2]) ProductLabel[L1, L2] {
	return ProductLabel[L1, L2]{
		Left:  p.Left.Join(other.Left),
		Right: p.Right.Join(other.Right),
	}
}

func (p ProductLabel[L1, L2]) Meet(other ProductLabel[L1, L2]) ProductLabel[L1, L2] {
	return ProductLabel[L1, L2]{
		Left:  p.Left.Meet(other.Left),
		Right: p.Right.Meet(other.Right),
	}
}

func (p ProductLabel[L1, L2]) String() string {
	return fmt.Sprintf("(%s, %s)", p.Left.String(), p.Right.String())
}

var ProductLabelLattice Lattice[ProductLabel[ConfidentialityLabel, IntegrityLabel]] = ProductLabel[ConfidentialityLabel, IntegrityLabel]{}

// InverseLattice inverts the order of an underlying lattice.
type InverseLattice[L Lattice[L]] struct {
	Inner L
}

func (i InverseLattice[L]) Leq(other InverseLattice[L]) bool {
	// Invert order: i <= other  ⇔  other.Inner <= i.Inner
	return other.Inner.Leq(i.Inner)
}

func (i InverseLattice[L]) Join(other InverseLattice[L]) InverseLattice[L] {
	// join in inverse is meet in the original
	return InverseLattice[L]{Inner: i.Inner.Meet(other.Inner)}
}

func (i InverseLattice[L]) Meet(other InverseLattice[L]) InverseLattice[L] {
	// meet in inverse is join in the original
	return InverseLattice[L]{Inner: i.Inner.Join(other.Inner)}
}

func (i InverseLattice[L]) String() string {
	return fmt.Sprintf("Inverse(%s)", i.Inner.String())
}

var _ Lattice[InverseLattice[ConfidentialityLabel]] = InverseLattice[ConfidentialityLabel]{}
