package lerobot

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"math"
	"os"
	"path/filepath"
)

// The statistics LeRobot normalizes with: per feature and dimension, with the quantiles it expects; folded across episodes the way its own aggregation does.
type featureStats struct {
	Min   []float64 `json:"min"`
	Max   []float64 `json:"max"`
	Mean  []float64 `json:"mean"`
	Std   []float64 `json:"std"`
	Count []int64   `json:"count"`
	Q01   []float64 `json:"q01"`
	Q10   []float64 `json:"q10"`
	Q50   []float64 `json:"q50"`
	Q90   []float64 `json:"q90"`
	Q99   []float64 `json:"q99"`
}

func vectorStats(rows []frameRow, pick func(frameRow) []float32) *featureStats {
	if len(rows) == 0 {
		return nil
	}
	dims := len(pick(rows[0]))
	columns := make([][]float64, dims)
	for _, row := range rows {
		for d, v := range pick(row) {
			columns[d] = append(columns[d], float64(v))
		}
	}
	return columnStats(columns, int64(len(rows)))
}

func columnStats(columns [][]float64, count int64) *featureStats {
	s := &featureStats{Count: []int64{count}}
	for _, values := range columns {
		sorted := sortedCopy(values)
		var sum, sumSquares float64
		for _, v := range values {
			sum += v
			sumSquares += v * v
		}
		n := float64(len(values))
		mean := sum / n
		s.Min = append(s.Min, sorted[0])
		s.Max = append(s.Max, sorted[len(sorted)-1])
		s.Mean = append(s.Mean, mean)
		s.Std = append(s.Std, math.Sqrt(math.Max(0, sumSquares/n-mean*mean)))
		s.Q01 = append(s.Q01, quantile(sorted, 0.01))
		s.Q10 = append(s.Q10, quantile(sorted, 0.10))
		s.Q50 = append(s.Q50, quantile(sorted, 0.50))
		s.Q90 = append(s.Q90, quantile(sorted, 0.90))
		s.Q99 = append(s.Q99, quantile(sorted, 0.99))
	}
	return s
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	position := q * float64(len(sorted)-1)
	low := int(math.Floor(position))
	high := int(math.Ceil(position))
	return sorted[low] + (sorted[high]-sorted[low])*(position-float64(low))
}

const picturesSampled = 50

// Pictures are sampled evenly and their pixels pooled per channel, scaled to 0..1, as LeRobot does for images; the count is the episode's frames, as there.
func pictureStats(pictures []picture, frames int) (*featureStats, error) {
	step := max(1, len(pictures)/picturesSampled)
	channels := make([][]float64, 3)
	for i := 0; i < len(pictures); i += step {
		data, err := os.ReadFile(pictures[i].file)
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y += 8 {
			for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
				r, g, b, _ := img.At(x, y).RGBA()
				channels[0] = append(channels[0], float64(r>>8)/255)
				channels[1] = append(channels[1], float64(g>>8)/255)
				channels[2] = append(channels[2], float64(b>>8)/255)
			}
		}
	}
	if len(channels[0]) == 0 {
		return nil, os.ErrNotExist
	}
	return columnStats(channels, int64(frames)), nil
}

func (s *Sink) foldStats(episodeStats map[string]*featureStats) error {
	path := filepath.Join(s.root, statsPath)
	total := map[string]*featureStats{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &total); err != nil {
			return err
		}
	}
	for key, stats := range episodeStats {
		if stats == nil {
			continue
		}
		if existing, ok := total[key]; ok {
			total[key] = fold(existing, stats)
		} else {
			total[key] = stats
		}
	}
	return writeJSON(path, total)
}

func fold(a, b *featureStats) *featureStats {
	na, nb := float64(a.Count[0]), float64(b.Count[0])
	n := na + nb
	out := &featureStats{Count: []int64{a.Count[0] + b.Count[0]}}
	for d := range a.Mean {
		mean := (a.Mean[d]*na + b.Mean[d]*nb) / n
		variance := ((a.Std[d]*a.Std[d]+(a.Mean[d]-mean)*(a.Mean[d]-mean))*na + (b.Std[d]*b.Std[d]+(b.Mean[d]-mean)*(b.Mean[d]-mean))*nb) / n
		out.Min = append(out.Min, math.Min(a.Min[d], b.Min[d]))
		out.Max = append(out.Max, math.Max(a.Max[d], b.Max[d]))
		out.Mean = append(out.Mean, mean)
		out.Std = append(out.Std, math.Sqrt(variance))
		out.Q01 = append(out.Q01, math.Min(a.Q01[d], b.Q01[d]))
		out.Q10 = append(out.Q10, math.Min(a.Q10[d], b.Q10[d]))
		out.Q50 = append(out.Q50, math.Min(a.Q50[d], b.Q50[d]))
		out.Q90 = append(out.Q90, math.Max(a.Q90[d], b.Q90[d]))
		out.Q99 = append(out.Q99, math.Max(a.Q99[d], b.Q99[d]))
	}
	return out
}
