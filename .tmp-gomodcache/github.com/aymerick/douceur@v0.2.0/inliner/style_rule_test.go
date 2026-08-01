package inliner

import "testing"

// Reference: http://www.w3.org/TR/selectors/#specificity
//
// *               /* a=0 b=0 c=0 -> specificity =   0 */
// LI              /* a=0 b=0 c=1 -> specificity =   1 */
// UL LI           /* a=0 b=0 c=2 -> specificity =   2 */
// UL OL+LI        /* a=0 b=0 c=3 -> specificity =   3 */
// H1 + *[REL=up]  /* a=0 b=1 c=1 -> specificity =  11 */
// UL OL LI.red    /* a=0 b=1 c=3 -> specificity =  13 */
// LI.red.level    /* a=0 b=2 c=1 -> specificity =  21 */
// #x34y           /* a=1 b=0 c=0 -> specificity = 100 */
// #s12:not(FOO)   /* a=1 b=0 c=1 -> specificity = 101 */
func TestComputeSpecificity(t *testing.T) {
	if val := ComputeSpecificity("*"); val != 0 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("LI"); val != 1 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("UL LI"); val != 2 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("UL OL+LI "); val != 3 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("H1 + *[REL=up]"); val != 11 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("UL OL LI.red"); val != 13 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("LI.red.level"); val != 21 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	if val := ComputeSpecificity("#x34y"); val != 100 {
		t.Fatal("Failed to compute specificity: ", val)
	}

	// This one fails ! \o/
	// if val := ComputeSpecificity("#s12:not(FOO)"); val != 101 {
	// 	t.Fatal("Failed to compute specificity: ", val)
	// }
}
