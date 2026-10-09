package numa

import "testing"

func TestPlacementPresenceIsExplicit(t *testing.T) {
	if (Placement{}).Validate() != nil {
		t.Fatal("zero placement should be unspecified")
	}
	if On(0).Validate() != nil || !On(0).Present {
		t.Fatal("explicit node zero invalid")
	}
	if (Placement{Node: 4}).Validate() != ErrInvalidPlacement {
		t.Fatal("nonzero absent node accepted")
	}
}
