# 框架层参考数据

这些 CSV 是 `internal/analysis/who_bmi_data.go` 与 `internal/analysis/framework_reference.go` 的源数据。
修改 CSV 后运行：

```bash
go generate ./internal/analysis
```

生成器会校验行数、月份连续性、数值范围，并重新生成两个 Go 文件。CI 运行
`go generate ./internal/analysis && git diff --exit-code`，防止 CSV 与生成物漂移。

## who_bmi_boys.csv / who_bmi_girls.csv

- 来源：WHO 2007 BMI-for-age 5–19 years expanded tables（LMS）。
- URL：https://www.who.int/tools/growth-reference-data-for-5to19-years/indicators/bmi-for-age
- 列：`month`（61–228，含端点）、`l`、`m`、`s`。
- 行数：每个性别 168 行。

## wst586_cutoffs.csv

- 来源：WS/T 586—2018 表 1（6–18 岁性别年龄别 BMI 超重 / 肥胖界值）。
- URL：https://www.nhc.gov.cn/wjw/pqt/201803/a7962d1ac01647b9837110bfd2d69b26/files/1739781209962_63132.pdf
- 列：`age_months`（72–216，步长 6）、`male_overweight`、`male_obese`、
  `female_overweight`、`female_obese`；BMI 单位 kg/m²。
- 行数：25 行。

CSV 只存数据，不写注释；来源、版本和列语义放在本文件。生成器不会联网，
更新标准时先人工核对源表，再替换 CSV。
