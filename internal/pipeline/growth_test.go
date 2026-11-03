package pipeline_test

import (
	"errors"
	"testing"
	"time"

	"healthyduoduo/internal/pipeline"
)

func f64(v float64) *float64 { return &v }

func TestValidateGrowthProfile(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	valid := pipeline.GrowthProfile{MeasuredOn: "2026-10-01", HeightCm: f64(118.5), WeightKg: f64(22.4), Source: pipeline.GrowthSourceMeasured}
	if err := pipeline.ValidateGrowthProfile(valid, now); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}

	heightOnly := valid
	heightOnly.WeightKg = nil
	if err := pipeline.ValidateGrowthProfile(heightOnly, now); err != nil {
		t.Errorf("height-only profile rejected: %v", err)
	}
	weightOnly := valid
	weightOnly.HeightCm = nil
	if err := pipeline.ValidateGrowthProfile(weightOnly, now); err != nil {
		t.Errorf("weight-only profile rejected: %v", err)
	}

	cases := []struct {
		name string
		in   pipeline.GrowthProfile
		want error
	}{
		{"missing measurement", pipeline.GrowthProfile{MeasuredOn: "2026-10-01", Source: pipeline.GrowthSourceMeasured}, pipeline.ErrGrowthMeasurementMissing},
		{"bad date", pipeline.GrowthProfile{MeasuredOn: "2026-02-30", HeightCm: f64(120), Source: pipeline.GrowthSourceMeasured}, pipeline.ErrGrowthMeasuredOnInvalid},
		{"future date", pipeline.GrowthProfile{MeasuredOn: "2026-11-04", HeightCm: f64(120), Source: pipeline.GrowthSourceMeasured}, pipeline.ErrGrowthMeasuredOnInvalid},
		{"bad source", pipeline.GrowthProfile{MeasuredOn: "2026-10-01", HeightCm: f64(120), Source: "scale"}, pipeline.ErrGrowthSourceInvalid},
		{"zero height", pipeline.GrowthProfile{MeasuredOn: "2026-10-01", HeightCm: f64(0), Source: pipeline.GrowthSourceMeasured}, pipeline.ErrGrowthMeasurementInvalid},
		{"negative weight", pipeline.GrowthProfile{MeasuredOn: "2026-10-01", WeightKg: f64(-1), Source: pipeline.GrowthSourceMeasured}, pipeline.ErrGrowthMeasurementInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := pipeline.ValidateGrowthProfile(c.in, now); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestBMI(t *testing.T) {
	cases := []struct {
		name     string
		heightCm *float64
		weightKg *float64
		want     *float64
	}{
		{"normal", f64(118.5), f64(22.4), f64(16)},
		{"one decimal", f64(120), f64(25), f64(17.4)},
		{"missing height", nil, f64(20), nil},
		{"missing weight", f64(120), nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pipeline.BMI(c.heightCm, c.weightKg)
			if c.want == nil {
				if got != nil {
					t.Fatalf("BMI = %v, want nil", *got)
				}
				return
			}
			if got == nil || *got != *c.want {
				t.Fatalf("BMI = %v, want %v", got, *c.want)
			}
		})
	}
}
