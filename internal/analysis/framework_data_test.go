package analysis

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestWHODataMatchesCSV(t *testing.T) {
	cases := []struct {
		path string
		rows []whoBMIRow
	}{
		{"data/who_bmi_boys.csv", whoBMIBoys},
		{"data/who_bmi_girls.csv", whoBMIGirls},
	}
	for _, testCase := range cases {
		records := readFrameworkCSV(t, testCase.path, []string{"month", "l", "m", "s"})
		if len(records) != len(testCase.rows) {
			t.Fatalf("%s: %d rows, Go has %d", testCase.path, len(records), len(testCase.rows))
		}
		for index, record := range records {
			month := frameworkCSVInt(t, testCase.path, index+2, record[0])
			l := frameworkCSVFloat(t, testCase.path, index+2, record[1])
			m := frameworkCSVFloat(t, testCase.path, index+2, record[2])
			s := frameworkCSVFloat(t, testCase.path, index+2, record[3])
			row := testCase.rows[index]
			if row.month != month || row.l != l || row.m != m || row.s != s {
				t.Fatalf("%s:%d: CSV {%d %v %v %v} != Go {%d %v %v %v}",
					testCase.path, index+2, month, l, m, s, row.month, row.l, row.m, row.s)
			}
		}
	}
}

func TestWST586DataMatchesCSV(t *testing.T) {
	header := []string{"age_months", "male_overweight", "male_obese", "female_overweight", "female_obese"}
	records := readFrameworkCSV(t, "data/wst586_cutoffs.csv", header)
	if len(records) != len(wst586Cutoffs) {
		t.Fatalf("wst586_cutoffs.csv: %d rows, Go has %d", len(records), len(wst586Cutoffs))
	}
	for index, record := range records {
		row := wst586Cutoff{
			ageMonths:        frameworkCSVInt(t, "data/wst586_cutoffs.csv", index+2, record[0]),
			maleOverweight:   frameworkCSVFloat(t, "data/wst586_cutoffs.csv", index+2, record[1]),
			maleObese:        frameworkCSVFloat(t, "data/wst586_cutoffs.csv", index+2, record[2]),
			femaleOverweight: frameworkCSVFloat(t, "data/wst586_cutoffs.csv", index+2, record[3]),
			femaleObese:      frameworkCSVFloat(t, "data/wst586_cutoffs.csv", index+2, record[4]),
		}
		if row != wst586Cutoffs[index] {
			t.Fatalf("wst586_cutoffs.csv:%d: CSV %+v != Go %+v", index+2, row, wst586Cutoffs[index])
		}
	}
}

func TestFrameworkDataCoverage(t *testing.T) {
	if len(whoBMIBoys) != 168 || len(whoBMIGirls) != 168 {
		t.Fatalf("WHO rows = %d/%d, want 168/168", len(whoBMIBoys), len(whoBMIGirls))
	}
	if len(wst586Cutoffs) != 25 {
		t.Fatalf("WST rows = %d, want 25", len(wst586Cutoffs))
	}
	for index := range whoBMIBoys {
		if got := whoBMIBoys[index].month; got != 61+index {
			t.Fatalf("boys[%d].month = %d, want %d", index, got, 61+index)
		}
		if got := whoBMIGirls[index].month; got != 61+index {
			t.Fatalf("girls[%d].month = %d, want %d", index, got, 61+index)
		}
		assertWHOFinite(t, "boys", index, whoBMIBoys[index])
		assertWHOFinite(t, "girls", index, whoBMIGirls[index])
	}
	for index := range wst586Cutoffs {
		row := wst586Cutoffs[index]
		if got, want := row.ageMonths, 72+index*6; got != want {
			t.Fatalf("wst[%d].ageMonths = %d, want %d", index, got, want)
		}
		if row.maleOverweight > row.maleObese || row.femaleOverweight > row.femaleObese {
			t.Fatalf("wst[%d] overweight > obese: %+v", index, row)
		}
	}
}

func TestFrameworkDataAnchors(t *testing.T) {
	assertWHORow(t, "boys[0]", whoBMIBoys[0], whoBMIRow{61, -0.7387, 15.2641, 0.0839})
	assertWHORow(t, "boys[167]", whoBMIBoys[167], whoBMIRow{228, -0.8419, 22.1883, 0.12948})
	assertWHORow(t, "girls[0]", whoBMIGirls[0], whoBMIRow{61, -0.8886, 15.2441, 0.09692})
	assertWHORow(t, "girls[167]", whoBMIGirls[167], whoBMIRow{228, -0.7496, 21.4269, 0.14441})
	if got, want := wst586Cutoffs[0], (wst586Cutoff{72, 16.4, 17.7, 16.2, 17.5}); got != want {
		t.Fatalf("wst[0] = %+v, want %+v", got, want)
	}
	if got, want := wst586Cutoffs[24], (wst586Cutoff{216, 24, 28, 24, 28}); got != want {
		t.Fatalf("wst[24] = %+v, want %+v", got, want)
	}
}

func assertWHOFinite(t *testing.T, name string, index int, row whoBMIRow) {
	t.Helper()
	if math.IsNaN(row.l) || math.IsInf(row.l, 0) ||
		math.IsNaN(row.m) || math.IsInf(row.m, 0) ||
		math.IsNaN(row.s) || math.IsInf(row.s, 0) {
		t.Fatalf("%s[%d] non-finite: %+v", name, index, row)
	}
	if row.m <= 0 || row.s <= 0 {
		t.Fatalf("%s[%d] non-positive M/S: %+v", name, index, row)
	}
}

func assertWHORow(t *testing.T, name string, got, want whoBMIRow) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %+v, want %+v", name, got, want)
	}
}

func readFrameworkCSV(t *testing.T, path string, header []string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(records) == 0 || len(records[0]) != len(header) {
		t.Fatalf("%s: bad header %v", path, records)
	}
	for index := range header {
		if records[0][index] != header[index] {
			t.Fatalf("%s: header[%d] = %q, want %q", path, index, records[0][index], header[index])
		}
	}
	for index, record := range records[1:] {
		if len(record) != len(header) {
			t.Fatalf("%s:%d: %d columns, want %d", path, index+2, len(record), len(header))
		}
	}
	return records[1:]
}

func frameworkCSVInt(t *testing.T, path string, line int, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("%s:%d: parse int %q: %v", path, line, value, err)
	}
	return parsed
}

func frameworkCSVFloat(t *testing.T, path string, line int, value string) float64 {
	t.Helper()
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("%s:%d: parse float %q: %v", path, line, value, err)
	}
	return parsed
}
