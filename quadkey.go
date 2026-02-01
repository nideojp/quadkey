package quadkey

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

const MERCATOR_MAX_LAT = 85.05112878

// --------------------------
// struct QuadKey
// --------------------------

type QuadKey string

func (key QuadKey) String() string {
	return string(key)
}

func (key QuadKey) Valid() error {
	if key == "" {
		return errors.New("key is empty")
	}
	for i := 0; i < len(key); i++ {
		switch key[i] {
		case '0', '1', '2', '3':
			// ok
		default:
			return fmt.Errorf("key contains invalid digit at index %d: %q", i, key[i])
		}
	}
	return nil
}

func (key QuadKey) Z() int {
	return len(key)
}

func (key QuadKey) XYZ() (x, y, z int) {
	if err := key.Valid(); err != nil {
		return -1, -1, -1
	}

	x, y = 0, 0
	z = key.Z()
	for i := z; i > 0; i-- {
		mask := 1 << (i - 1)
		switch key[z-i] {
		case '0':
			// no-op
		case '1':
			x |= mask
		case '2':
			y |= mask
		case '3':
			x |= mask
			y |= mask
		default:
			return -1, -1, -1
		}
	}
	return
}

func (key QuadKey) Parent() (QuadKey, error) {
	if err := key.Valid(); err != nil {
		return "", err
	}
	z := key.Z()
	if z == 1 {
		return "", errors.New("key is root")
	}
	parent := key[:z-1]
	return QuadKey(parent), nil
}

func (key QuadKey) Children() []QuadKey {
	if err := key.Valid(); err != nil {
		return []QuadKey{}
	}

	x, y, z := key.XYZ()
	if x < 0 || y < 0 || z < 0 {
		return []QuadKey{}
	}

	// Child tiles at zoom z+1 are the 2x2 subdivision of the parent tile.
	// The order corresponds to appending digits 0,1,2,3.
	cx, cy := x<<1, y<<1
	zz := z + 1
	return []QuadKey{
		FromXYZ(cx, cy, zz),     // 0
		FromXYZ(cx+1, cy, zz),   // 1
		FromXYZ(cx, cy+1, zz),   // 2
		FromXYZ(cx+1, cy+1, zz), // 3
	}
}

func (key QuadKey) Bound() orb.Bound {
	if err := key.Valid(); err != nil {
		return orb.Bound{}
	}

	x, y, z := key.XYZ()
	if x < 0 || y < 0 || z < 0 {
		return orb.Bound{}
	}

	n := math.Pow(2.0, float64(z))

	west := float64(x)/n*360 - 180
	east := float64(x+1)/n*360 - 180
	north := math.Atan(math.Sinh(math.Pi*(1-2*float64(y)/n))) * 180 / math.Pi
	south := math.Atan(math.Sinh(math.Pi*(1-2*float64(y+1)/n))) * 180 / math.Pi

	diagonal := orb.LineString{
		{west, north},
		{east, south},
	}
	return diagonal.Bound()
}

func (key QuadKey) MarshalJSON() ([]byte, error) {
	value := string(key)
	return json.Marshal(&value)
}

func (key *QuadKey) UnmarshalJSON(data []byte) error {
	value := ""
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*key = QuadKey(value)
	return nil
}

func (key *QuadKey) ToPolygon() orb.Polygon {
	bound := key.Bound()
	return bound.ToPolygon()
}

func (key *QuadKey) ToFeature() *geojson.Feature {
	feature := geojson.NewFeature(key.ToPolygon())
	feature.ID = key.String()
	return feature
}

// --------------------------
// internal function's
// --------------------------

func normalize(lon, lat float64) (float64, float64) {
	// Normalize longitude to [-180, 180] (keeping 180 as 180).
	// This avoids mapping +180 to -180, which can cause dateline surprises.
	lon = math.Mod(lon, 360)
	if lon <= -180 {
		lon += 360
	} else if lon > 180 {
		lon -= 360
	}

	// Web mercator latitude clamp
	if lat > MERCATOR_MAX_LAT {
		lat = MERCATOR_MAX_LAT
	} else if lat < -MERCATOR_MAX_LAT {
		lat = -MERCATOR_MAX_LAT
	}
	return lon, lat
}

func toX(lon float64, z int) int {
	n := math.Exp2(float64(z))
	x := math.Floor((lon + 180) / 360 * n)
	return int(math.Max(0, math.Min(x, n-1)))
}

func toY(lat float64, z int) int {
	n := math.Exp2(float64(z))
	rad := lat * math.Pi / 180
	y := math.Floor((1 - math.Log(math.Tan(rad)+1/math.Cos(rad))/math.Pi) / 2 * n)
	return int(math.Max(0, math.Min(y, n-1)))
}

// --------------------------
// global function's
// --------------------------

func FromXYZ(x, y, z int) QuadKey {
	quadkey := make([]byte, z)
	for i := z; i > 0; i-- {
		digit := byte('0')
		mask := 1 << (i - 1)
		if (x & mask) != 0 {
			digit++
		}
		if (y & mask) != 0 {
			digit += 2
		}
		quadkey[z-i] = digit
	}
	return QuadKey(quadkey)
}

func FromLonLat(lon, lat float64, zoom int) QuadKey {
	lon, lat = normalize(lon, lat)
	x := toX(lon, zoom)
	y := toY(lat, zoom)
	return FromXYZ(x, y, zoom)
}

func FromPoint(point orb.Point, zoom int) QuadKey {
	lon, lat := normalize(point.Lon(), point.Lat())
	x := toX(lon, zoom)
	y := toY(lat, zoom)
	return FromXYZ(x, y, zoom)
}

func FromKey(key string) (QuadKey, error) {
	quadkey := QuadKey(key)
	if err := quadkey.Valid(); err != nil {
		return "", err
	}
	return quadkey, nil
}

func KeysInBound(bound orb.Bound, zoom int) []QuadKey {
	west, south := normalize(bound.Left(), bound.Bottom())
	east, north := normalize(bound.Right(), bound.Top())

	// Treat bounds as half-open intervals in tile/grid terms:
	//   lon in [west, east), lat in [south, north)
	// This prevents "extra tiles" when the max edges land exactly on a tile boundary.
	//
	// We realize the half-open behavior by nudging the max edges one float toward the interior.
	// (If west==east or south==north, this can result in an empty set as expected for a zero-area bound.)
	eastIn := east
	southIn := south
	if eastIn != west {
		// Move east slightly toward west (interior for lon).
		eastIn = math.Nextafter(eastIn, west)
	}
	if southIn != north {
		// Move south slightly toward north (interior for lat).
		southIn = math.Nextafter(southIn, north)
	}

	minX := toX(west, zoom)
	maxX := toX(eastIn, zoom)
	minY := toY(north, zoom)
	maxY := toY(southIn, zoom)

	// If the bound is inverted or crosses the dateline, normalization can produce min>max.
	// We keep the current behavior by swapping, but callers that require dateline-aware
	// coverage should split the bound at the dateline before calling.
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	// Half-open bounds can legitimately produce an empty set (e.g. zero width/height).
	if maxX < minX || maxY < minY {
		return []QuadKey{}
	}

	keys := make([]QuadKey, 0, (maxX-minX+1)*(maxY-minY+1))
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			keys = append(keys, FromXYZ(x, y, zoom))
		}
	}
	return keys
}

func ToFeatureCollection(keys ...QuadKey) *geojson.FeatureCollection {
	collection := geojson.NewFeatureCollection()
	for _, key := range keys {
		collection.Append(key.ToFeature())
	}
	return collection
}
