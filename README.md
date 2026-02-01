# quadkey

A Go library for working with **Bing Maps QuadKeys**, providing conversion between  
**longitude/latitude**, **XYZ tiles**, **QuadKeys**, and **GeoJSON geometries**.

This package is designed for **GIS**, **map tiling**, **weather grids**, and  
**geospatial indexing** workflows.

---

## Features

- QuadKey ↔ XYZ tile conversion
- Lon/Lat → QuadKey (Web Mercator)
- Parent / children QuadKey traversal
- Tile boundary calculation
- QuadKey → orb.Polygon
- QuadKey → GeoJSON Feature / FeatureCollection
- JSON marshal / unmarshal support
- Compatible with Bing Maps QuadKey specification
- `KeysInBound` returns all QuadKeys covering a bounding box using half-open bounds ([west, east), [south, north))

---

## Installation

```bash
go get github.com/nideojp/go-quadkey
```

Dependencies:

- github.com/paulmach/orb
- github.com/paulmach/orb/geojson

---

## Basic Usage

### Create a QuadKey from Lon / Lat

```go
qk := quadkey.FromLonLat(139.767125, 35.681236, 8)
fmt.Println(qk.String())
```

Output example:

```text
13300221
```

---

### Create a QuadKey from XYZ Tile

```go
qk := quadkey.FromXYZ(215, 100, 8)
fmt.Println(qk)
```

---

### Parse an Existing QuadKey

```go
qk, err := quadkey.FromKey("13300221")
if err != nil {
  log.Fatal(err)
}
```

---

## QuadKey Properties

### Zoom Level

```go
z := qk.Z()
```

---

### Convert to XYZ

```go
x, y, z := qk.XYZ()
fmt.Println(x, y, z)
```

---

### Parent QuadKey

```go
parent, err := qk.Parent()
if err != nil {
  log.Fatal(err)
}
```

---

### Children QuadKeys

Returns all child QuadKeys at the next zoom level.

```go
children := qk.Children()
for _, c := range children {
  fmt.Println(c)
}
```

**Note:** `Children()` always returns exactly 4 QuadKeys for a valid key, corresponding to digits 0, 1, 2, and 3.

---

## Spatial Operations

### Tile Boundary

Returns the geographic bounding box of the QuadKey.

```go
bound := qk.Bound()
fmt.Println(bound.Left(), bound.Bottom(), bound.Right(), bound.Top())
```

---

### Convert QuadKey to Polygon

```go
polygon := qk.ToPolygon()
```

The polygon represents the tile area in lon/lat (EPSG:4326).

---

### Convert QuadKey to GeoJSON Feature

```go
feature := qk.ToFeature()
feature.ID // QuadKey string
```

---

### FeatureCollection from Multiple QuadKeys

```go
collection := quadkey.ToFeatureCollection(qk1, qk2, qk3)
```

---

## Working with Bounds

### Generate QuadKeys Inside a Bounding Box

```go
bound := orb.Bound{
  Min: orb.Point{139.5, 35.5},
  Max: orb.Point{140.0, 36.0},
}

keys := quadkey.KeysInBound(bound, 8)
```

This is useful for:

- Tile-based data extraction
- Weather / raster grids
- Spatial indexing

#### Bounds semantics

**Q:** Why use half-open bounds in `KeysInBound`?  
**A:** Half-open bounds prevent returning extra tiles when the bounding box edges align exactly with tile boundaries.

- `KeysInBound(bound, zoom)` treats the bound as **half-open** in tile/grid space:  
  longitude in `[west, east)`, latitude in `[south, north)`
- This avoids returning extra tiles when the max edges align exactly with tile boundaries.
- For regions that cross the dateline, split the region before calling `KeysInBound`.

##### Dateline split example (pseudo-code)

```go
if bound.Min[0] > bound.Max[0] { // crosses dateline
  westBound := orb.Bound{
    Min: orb.Point{bound.Min[0], bound.Min[1]},
    Max: orb.Point{180.0, bound.Max[1]},
  }
  eastBound := orb.Bound{
    Min: orb.Point{-180.0, bound.Min[1]},
    Max: orb.Point{bound.Max[0], bound.Max[1]},
  }
  keysWest := quadkey.KeysInBound(westBound, zoom)
  keysEast := quadkey.KeysInBound(eastBound, zoom)
  keys := append(keysWest, keysEast...)
}
```

---

## JSON Support

### Marshal

```go
data, _ := json.Marshal(qk)
```

```json
"13300221"
```

---

### Unmarshal

```go
var qk quadkey.QuadKey
json.Unmarshal([]byte(`"13300221"`), &qk)
```

---

## Coordinate System Notes

- Uses Web Mercator projection
- Latitude is clamped to ±85.05112878°
- Longitude is normalized to −180° to +180° (with +180° remaining +180°, not normalized to −180°)
- Output geometries are in WGS84 (EPSG:4326)

---

## How QuadKeys Work

Each QuadKey digit represents a quadrant of the tile:

| Digit | Quadrant |
|------:|----------|
| 0 | Upper-left |
| 1 | Upper-right |
| 2 | Lower-left |
| 3 | Lower-right |

Internally, each digit is derived from XYZ tile bits:

```go
digit = (x_bit << 1) | y_bit
```

---

## Typical Use Cases

- Map tile indexing
- Weather model grids
- Vector tile generation
- Spatial partitioning
- GeoJSON tile visualization
- QuadTree-based spatial search

---

## License

MIT License

---

## Related

- Bing Maps QuadKey system
- Web Mercator (EPSG:3857)
- orb Go geometry library
