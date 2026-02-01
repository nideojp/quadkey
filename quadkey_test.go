package quadkey

import (
	"encoding/json"
	"math"
	"sort"
	"testing"

	"github.com/paulmach/orb"
)

// helper to compare ints
func assertEqualInt(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %d, want %d", name, got, want)
	}
}

func TestQuadKeyValid(t *testing.T) {
	tests := []struct {
		name    string
		key     QuadKey
		wantErr bool
	}{
		{"empty", QuadKey(""), true},
		{"ok_single_0", QuadKey("0"), false},
		{"ok_mixed", QuadKey("01230123"), false},
		{"bad_digit", QuadKey("01a3"), true},
		{"bad_symbol", QuadKey("01-3"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.key.Valid()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestFromXYZAndXYZRoundTrip(t *testing.T) {
	// test a handful of x/y points across multiple zoom levels
	type tc struct {
		x, y, z int
	}
	cases := []tc{
		{0, 0, 1},
		{1, 1, 1},
		{0, 1, 2},
		{2, 1, 2},
		{3, 0, 2},
		{10, 12, 5},
		{123, 456, 10},
	}

	for _, c := range cases {
		qk := FromXYZ(c.x, c.y, c.z)
		if err := qk.Valid(); err != nil {
			t.Fatalf("generated quadkey should be valid: %v", err)
		}
		x, y, z := qk.XYZ()
		assertEqualInt(t, "x", x, c.x)
		assertEqualInt(t, "y", y, c.y)
		assertEqualInt(t, "z", z, c.z)
	}
}

func TestXYZInvalidKey(t *testing.T) {
	x, y, z := QuadKey("01a3").XYZ()
	if x != -1 || y != -1 || z != -1 {
		t.Fatalf("expected (-1,-1,-1) for invalid key, got (%d,%d,%d)", x, y, z)
	}
}

func TestParent(t *testing.T) {
	_, err := QuadKey("").Parent()
	if err == nil {
		t.Fatalf("expected error for empty key")
	}

	_, err = QuadKey("0").Parent()
	if err == nil {
		t.Fatalf("expected error for root key")
	}

	p, err := QuadKey("0123").Parent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.String() != "012" {
		t.Fatalf("parent: got %q, want %q", p.String(), "012")
	}
}

func TestChildren(t *testing.T) {
	// children should be 4 tiles at the next zoom level
	k := QuadKey("12")
	children := k.Children()
	if len(children) != 4 {
		t.Fatalf("children length: got %d, want 4", len(children))
	}

	// expected children are key + each digit
	want := []string{"120", "121", "122", "123"}
	got := make([]string, 0, len(children))
	for _, c := range children {
		got = append(got, c.String())
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("children[%d]: got %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestBoundIsNonEmptyForValidKey(t *testing.T) {
	b := QuadKey("0").Bound()
	if b == (orb.Bound{}) {
		t.Fatalf("expected non-empty bound for valid key")
	}

	// Bounds should be within mercator limits and lon range.
	if b.Left() < -180.000001 || b.Right() > 180.000001 {
		t.Fatalf("lon out of range: left=%f right=%f", b.Left(), b.Right())
	}
	if b.Bottom() < -MERCATOR_MAX_LAT-1e-6 || b.Top() > MERCATOR_MAX_LAT+1e-6 {
		t.Fatalf("lat out of range: bottom=%f top=%f", b.Bottom(), b.Top())
	}

	// width/height should be positive
	if !(b.Right() > b.Left()) || !(b.Top() > b.Bottom()) {
		t.Fatalf("expected positive area bound: %+v", b)
	}
}

func TestBoundInvalidKeyReturnsEmpty(t *testing.T) {
	b := QuadKey("01a3").Bound()
	if b != (orb.Bound{}) {
		t.Fatalf("expected empty bound for invalid key, got %+v", b)
	}
}

func TestFromLonLatClampsLatitude(t *testing.T) {
	// latitude above mercator max should be clamped and not produce NaN bounds
	qk := FromLonLat(0, 90, 3)
	if err := qk.Valid(); err != nil {
		t.Fatalf("quadkey should be valid: %v", err)
	}
	b := qk.Bound()
	if b == (orb.Bound{}) {
		t.Fatalf("expected non-empty bound")
	}
	if math.IsNaN(b.Top()) || math.IsNaN(b.Bottom()) || math.IsNaN(b.Left()) || math.IsNaN(b.Right()) {
		t.Fatalf("bound contains NaN: %+v", b)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	orig := QuadKey("13300221")
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded QuadKey
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != orig {
		t.Fatalf("round trip mismatch: got %q, want %q", decoded.String(), orig.String())
	}
}

func TestKeysInBoundContainsExpectedKey(t *testing.T) {
	// Pick a key, use its bound, ensure KeysInBound at same zoom includes it.
	key := QuadKey("13300221")
	bound := key.Bound()

	keys := KeysInBound(bound, key.Z())
	if len(keys) == 0 {
		t.Fatalf("expected some keys")
	}

	found := false
	for _, k := range keys {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected KeysInBound to include %q, got %d keys", key.String(), len(keys))
	}
}

func TestToFeatureCollection(t *testing.T) {
	k1 := QuadKey("0")
	k2 := QuadKey("1")

	fc := ToFeatureCollection(k1, k2)
	if fc == nil {
		t.Fatalf("feature collection is nil")
	}
	if len(fc.Features) != 2 {
		t.Fatalf("feature count: got %d, want 2", len(fc.Features))
	}
	if fc.Features[0].ID != k1.String() {
		t.Fatalf("feature[0].ID: got %v, want %v", fc.Features[0].ID, k1.String())
	}
	if fc.Features[1].ID != k2.String() {
		t.Fatalf("feature[1].ID: got %v, want %v", fc.Features[1].ID, k2.String())
	}
}
