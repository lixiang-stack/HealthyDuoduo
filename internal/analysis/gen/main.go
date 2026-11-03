// 生成框架层参考数据 Go 文件:CSV 是唯一源,生成物编译期使用。
// 运行: go generate ./internal/analysis
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"go/format"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	whoSource = "https://www.who.int/tools/growth-reference-data-for-5to19-years/indicators/bmi-for-age"
	wstSource = "https://www.nhc.gov.cn/wjw/pqt/201803/a7962d1ac01647b9837110bfd2d69b26/files/1739781209962_63132.pdf"
)

type whoRow struct {
	month int
	l     float64
	m     float64
	s     float64
}

type wstRow struct {
	ageMonths        int
	maleOverweight   float64
	maleObese        float64
	femaleOverweight float64
	femaleObese      float64
}

func main() {
	dataDir := flag.String("data", "data", "CSV source directory")
	outDir := flag.String("out", ".", "generated Go output directory")
	flag.Parse()

	boys := readWHO(filepath.Join(*dataDir, "who_bmi_boys.csv"), 61)
	girls := readWHO(filepath.Join(*dataDir, "who_bmi_girls.csv"), 61)
	cutoffs := readWST(filepath.Join(*dataDir, "wst586_cutoffs.csv"), 72)

	writeGo(filepath.Join(*outDir, "who_bmi_data.go"), renderWHO(boys, girls))
	writeGo(filepath.Join(*outDir, "framework_reference.go"), renderWST(cutoffs))
	fmt.Printf("generated %d boys + %d girls WHO rows, %d WST rows\n", len(boys), len(girls), len(cutoffs))
}

func readWHO(path string, startMonth int) []whoRow {
	records := readCSV(path, []string{"month", "l", "m", "s"})
	const wantRows = 168
	if len(records) != wantRows {
		fatal("%s: got %d rows, want %d", path, len(records), wantRows)
	}
	rows := make([]whoRow, 0, len(records))
	for index, record := range records {
		line := index + 2
		month := parseInt(record[0], path, line)
		if want := startMonth + index; month != want {
			fatal("%s:%d: month = %d, want %d", path, line, month, want)
		}
		l := parseFloat(record[1], path, line)
		m := parseFloat(record[2], path, line)
		s := parseFloat(record[3], path, line)
		if l < -5 || l > 5 {
			fatal("%s:%d: L = %v outside [-5, 5]", path, line, l)
		}
		if m < 5 || m > 40 {
			fatal("%s:%d: M = %v outside [5, 40]", path, line, m)
		}
		if s < 0.01 || s > 0.5 {
			fatal("%s:%d: S = %v outside [0.01, 0.5]", path, line, s)
		}
		rows = append(rows, whoRow{month: month, l: l, m: m, s: s})
	}
	return rows
}

func readWST(path string, startMonth int) []wstRow {
	header := []string{"age_months", "male_overweight", "male_obese", "female_overweight", "female_obese"}
	records := readCSV(path, header)
	const wantRows = 25
	if len(records) != wantRows {
		fatal("%s: got %d rows, want %d", path, len(records), wantRows)
	}
	rows := make([]wstRow, 0, len(records))
	for index, record := range records {
		line := index + 2
		ageMonths := parseInt(record[0], path, line)
		if want := startMonth + index*6; ageMonths != want {
			fatal("%s:%d: age_months = %d, want %d", path, line, ageMonths, want)
		}
		row := wstRow{
			ageMonths:        ageMonths,
			maleOverweight:   parseFloat(record[1], path, line),
			maleObese:        parseFloat(record[2], path, line),
			femaleOverweight: parseFloat(record[3], path, line),
			femaleObese:      parseFloat(record[4], path, line),
		}
		for name, value := range map[string]float64{
			"male_overweight":   row.maleOverweight,
			"male_obese":        row.maleObese,
			"female_overweight": row.femaleOverweight,
			"female_obese":      row.femaleObese,
		} {
			if value < 5 || value > 50 {
				fatal("%s:%d: %s = %v outside [5, 50]", path, line, name, value)
			}
		}
		if row.maleOverweight > row.maleObese {
			fatal("%s:%d: male_overweight > male_obese", path, line)
		}
		if row.femaleOverweight > row.femaleObese {
			fatal("%s:%d: female_overweight > female_obese", path, line)
		}
		rows = append(rows, row)
	}
	return rows
}

func readCSV(path string, header []string) [][]string {
	file, err := os.Open(path)
	if err != nil {
		fatal("open %s: %v", path, err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	if len(records) == 0 {
		fatal("%s: empty CSV", path)
	}
	if !slices.Equal(records[0], header) {
		fatal("%s: header = %v, want %v", path, records[0], header)
	}
	for index, record := range records[1:] {
		if len(record) != len(header) {
			fatal("%s:%d: got %d columns, want %d", path, index+2, len(record), len(header))
		}
	}
	return records[1:]
}

func parseInt(value, path string, line int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		fatal("%s:%d: parse int %q: %v", path, line, value, err)
	}
	return parsed
}

func parseFloat(value, path string, line int) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		fatal("%s:%d: parse float %q: %v", path, line, value, err)
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		fatal("%s:%d: non-finite float %q", path, line, value)
	}
	return parsed
}

func renderWHO(boys, girls []whoRow) []byte {
	var builder strings.Builder
	builder.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	builder.WriteString("// Source: " + whoSource + "\n")
	builder.WriteString("// CSV: internal/analysis/data/who_bmi_{boys,girls}.csv (month,l,m,s; 61..228).\n")
	builder.WriteString("package analysis\n\n")
	builder.WriteString("type whoBMIRow struct {\n\tmonth int\n\tl     float64\n\tm     float64\n\ts     float64\n}\n\n")
	renderWHOTable(&builder, "whoBMIBoys", boys)
	builder.WriteString("\n")
	renderWHOTable(&builder, "whoBMIGirls", girls)
	return []byte(builder.String())
}

func renderWHOTable(builder *strings.Builder, name string, rows []whoRow) {
	fmt.Fprintf(builder, "var %s = []whoBMIRow{\n", name)
	for _, row := range rows {
		fmt.Fprintf(builder, "\t{%d, %s, %s, %s},\n", row.month, formatFloat(row.l), formatFloat(row.m), formatFloat(row.s))
	}
	builder.WriteString("}\n")
}

func renderWST(rows []wstRow) []byte {
	var builder strings.Builder
	builder.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	builder.WriteString("// Source: WS/T 586—2018 表 1; " + wstSource + "\n")
	builder.WriteString("// CSV: internal/analysis/data/wst586_cutoffs.csv (age_months; 72..216 step 6).\n")
	builder.WriteString("package analysis\n\n")
	builder.WriteString("type wst586Cutoff struct {\n\tageMonths        int\n\tmaleOverweight   float64\n\tmaleObese        float64\n\tfemaleOverweight float64\n\tfemaleObese      float64\n}\n\n")
	builder.WriteString("var wst586Cutoffs = []wst586Cutoff{\n")
	for _, row := range rows {
		fmt.Fprintf(&builder, "\t{%d, %s, %s, %s, %s},\n", row.ageMonths,
			formatFloat(row.maleOverweight), formatFloat(row.maleObese),
			formatFloat(row.femaleOverweight), formatFloat(row.femaleObese))
	}
	builder.WriteString("}\n")
	return []byte(builder.String())
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func writeGo(path string, source []byte) {
	formatted, err := format.Source(source)
	if err != nil {
		fatal("format %s: %v", path, err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen framework data: "+format+"\n", args...)
	os.Exit(1)
}
