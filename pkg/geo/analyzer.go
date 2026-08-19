package geo

import (
	"math"
	"sync"
)

// Analyzer performs geographic analysis on CDR data
type Analyzer struct {
	mu          sync.RWMutex
	cellTowers  map[string]*CellTower
	radiusKm    float64
}

// CellTower represents a cell tower location
type CellTower struct {
	ID        string  `json:"id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	MCC       string  `json:"mcc"`
	MNC       string  `json:"mnc"`
	LAC       int     `json:"lac"`
	CID       int     `json:"cid"`
	Radius    float64 `json:"radius_km"`
	Carrier   string  `json:"carrier"`
}

// GeoFence represents a geographic boundary
type GeoFence struct {
	Name      string    `json:"name"`
	Center    GeoPoint  `json:"center"`
	RadiusKm  float64   `json:"radius_km"`
	Polygon   []GeoPoint `json:"polygon,omitempty"`
}

// GeoPoint represents a geographic coordinate
type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Movement represents a detected movement pattern
type Movement struct {
	FromTower  string    `json:"from_tower"`
	ToTower    string    `json:"to_tower"`
	FromGeo    GeoPoint  `json:"from_geo"`
	ToGeo      GeoPoint  `json:"to_geo"`
	DistanceKm float64   `json:"distance_km"`
	Duration   float64   `json:"duration_hours"`
	SpeedKmh   float64   `json:"speed_kmh"`
	Timestamp  string    `json:"timestamp"`
}

// GeoAnalysis holds geographic analysis results
type GeoAnalysis struct {
	TotalLocations    int                    `json:"total_locations"`
	UniqueTowers      int                    `json:"unique_towers"`
	MovementPatterns  []Movement             `json:"movement_patterns"`
	GeoFenceViolations []GeoFenceViolation   `json:"geo_fence_violations"`
	CoverageArea      float64                `json:"coverage_area_sq_km"`
	CenterPoint       GeoPoint               `json:"center_point"`
}

// GeoFenceViolation represents a breach of a geographic fence
type GeoFenceViolation struct {
	FenceName  string    `json:"fence_name"`
	Location   GeoPoint  `json:"location"`
	Timestamp  string    `json:"timestamp"`
	DistanceKm float64   `json:"distance_from_center_km"`
}

// NewAnalyzer creates a new geographic analyzer
func NewAnalyzer(radiusKm float64) *Analyzer {
	return &Analyzer{
		cellTowers: make(map[string]*CellTower),
		radiusKm:   radiusKm,
	}
}

// AddCellTower registers a cell tower location
func (a *Analyzer) AddCellTower(tower CellTower) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cellTowers[tower.ID] = &tower
}

// LoadCellTowers bulk loads cell tower data
func (a *Analyzer) LoadCellTowers(towers []CellTower) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tower := range towers {
		a.cellTowers[tower.ID] = &tower
	}
}

// AnalyzeMovements analyzes movement patterns between cell towers
func (a *Analyzer) AnalyzeMovements(towerSequence []string, timestamps []string) []Movement {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var movements []Movement

	for i := 1; i < len(towerSequence); i++ {
		fromTower, fromExists := a.cellTowers[towerSequence[i-1]]
		toTower, toExists := a.cellTowers[towerSequence[i]]

		if !fromExists || !toExists {
			continue
		}

		distance := HaversineDistance(
			fromTower.Latitude, fromTower.Longitude,
			toTower.Latitude, toTower.Longitude,
		)

		if distance > 0.1 {
			movements = append(movements, Movement{
				FromTower:  towerSequence[i-1],
				ToTower:    towerSequence[i],
				FromGeo:    GeoPoint{Latitude: fromTower.Latitude, Longitude: fromTower.Longitude},
				ToGeo:      GeoPoint{Latitude: toTower.Latitude, Longitude: toTower.Longitude},
				DistanceKm: distance,
			})
		}
	}

	return movements
}

// CheckGeoFence checks if locations violate a geographic fence
func (a *Analyzer) CheckGeoFence(fence GeoFence, locations []GeoPoint, timestamps []string) []GeoFenceViolation {
	var violations []GeoFenceViolation

	for i, loc := range locations {
		distance := HaversineDistance(
			fence.Center.Latitude, fence.Center.Longitude,
			loc.Latitude, loc.Longitude,
		)

		if distance > fence.RadiusKm {
			ts := ""
			if i < len(timestamps) {
				ts = timestamps[i]
			}
			violations = append(violations, GeoFenceViolation{
				FenceName:  fence.Name,
				Location:   loc,
				Timestamp:  ts,
				DistanceKm: distance,
			})
		}
	}

	return violations
}

// CalculateCoverageArea calculates the approximate coverage area
func (a *Analyzer) CalculateCoverageArea(locations []GeoPoint) float64 {
	if len(locations) < 3 {
		return 0
	}

	minLat, maxLat := locations[0].Latitude, locations[0].Latitude
	minLon, maxLon := locations[0].Longitude, locations[0].Longitude

	for _, loc := range locations {
		if loc.Latitude < minLat {
			minLat = loc.Latitude
		}
		if loc.Latitude > maxLat {
			maxLat = loc.Latitude
		}
		if loc.Longitude < minLon {
			minLon = loc.Longitude
		}
		if loc.Longitude > maxLon {
			maxLon = loc.Longitude
		}
	}

	width := HaversineDistance(minLat, minLon, minLat, maxLon)
	height := HaversineDistance(minLat, minLon, maxLat, minLon)

	return width * height
}

// FindCenterPoint calculates the center point of a set of locations
func (a *Analyzer) FindCenterPoint(locations []GeoPoint) GeoPoint {
	if len(locations) == 0 {
		return GeoPoint{}
	}

	sumLat, sumLon := 0.0, 0.0
	for _, loc := range locations {
		sumLat += loc.Latitude
		sumLon += loc.Longitude
	}

	return GeoPoint{
		Latitude:  sumLat / float64(len(locations)),
		Longitude: sumLon / float64(len(locations)),
	}
}

// HaversineDistance calculates distance between two points in km
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// GetCellTower returns cell tower by ID
func (a *Analyzer) GetCellTower(id string) *CellTower {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cellTowers[id]
}

// GetNearbyTowers returns towers within a radius
func (a *Analyzer) GetNearbyTowers(center GeoPoint, radiusKm float64) []*CellTower {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var nearby []*CellTower
	for _, tower := range a.cellTowers {
		dist := HaversineDistance(
			center.Latitude, center.Longitude,
			tower.Latitude, tower.Longitude,
		)
		if dist <= radiusKm {
			nearby = append(nearby, tower)
		}
	}
	return nearby
}
