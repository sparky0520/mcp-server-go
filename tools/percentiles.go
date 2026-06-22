package tools

import (
	"fmt"
	"sort"
)

type PercentilesArgs struct {
	Values []float64 `json:"values"`
}

type PercentilesResult struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
}

func Percentiles(args PercentilesArgs) (PercentilesResult, error) {
	if len(args.Values) == 0 {
		return PercentilesResult{}, fmt.Errorf("values must not be empty")
	}

	values := make([]float64, len(args.Values))
	copy(values, args.Values)
	sort.Float64s(values)

	var sum float64
	for _, v := range values {
		sum += v
	}

	return PercentilesResult{
		Count: len(values),
		Min:   values[0],
		P50:   percentile(values, 50),
		P95:   percentile(values, 95),
		P99:   percentile(values, 99),
		Max:   values[len(values)-1],
		Avg:   sum / float64(len(values)),
	}, nil
}

func percentile(values []float64, i int) float64 {
	return values[(i/100)*len(values)]
}
