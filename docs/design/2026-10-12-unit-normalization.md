# 单位规一化与 trend 质量门(2026-10-12)

## 为什么做

`./hdd trend 血小板` 的 5 个时序点中,2 个单位是 OCR 乱珠(`109/L`、`7/6.0`),
1 个 flag=unknown(low_confidence=true),delta 全 "-";golden 与库内报告大量
partial(24 张里 18 张)。逐层查证后的根因:

1. **单位乱珠 = 上标字形 + 源图质量**:印刷单位是上标形 `10⁹/L`,OCR 拍平成
   `109/L`(20 处);600px 微信截图(600×436,单位列仅数像素)读成 `7/6.0`;
   拍照件 blurry 读成 `1/6.0`。词典 units 早已注册多印形(`*10^9/L / 10~9/L / …`),
   但只是「接受匹配」,未做展示归一——同一 canonical 跨单印形不同,
   `sameUnit` 逐字节比较判不等,delta 全 "-";
2. **flag=unknown 行解析行为正确**:拍照件参考范围 `125--350` 被 OCR 读成
   `130--31`(上下颠倒),按「不猜」原则返回 unknown;该行 low_confidence=true
   的质量信号已落库,trend 查询未使用。
3. **partial 大量化主体是源图质量**(日期无法识别、value 格读不出),不是规则
   退化;但 trend 不区分项级质量,把不可信点与可信点平等并列 → 不可用。

## 做什么

**A. 识别侧单位规一化(python)**:新增数据文件 `recognizer/unit_families.yaml`
(单位族规范印形表:canonical + 已知损耗印形,证据规则与 *_dict.yaml 相同);
postprocess 在落报告时按该项词典注册集裁决归一 `_normalize_unit`——只替换
「该项 units 已注册 canonical」的印形,未注册乱珠保留原文(raw_text 留痕)。
`hdd show` 的单位不再随医院印法漂移,`sameUnit` 直接可比。

**B. trend 质量门(Go)**:`TrendRow` 增加 `low_confidence` 列;trend 默认排除
低置信或 flag=unknown 的点,`--all` 关闭过滤,被滤行数注明到 stderr。
不做整单 status 过滤(partial 单里单项可信,一刀切误杀)。

不在本期范围:换 OCR 模型/表结构识别(源图质量是瓶颈非模型);`rmol/L`、
`ag/L`、`ng/al`、`10/a1`、`IU/al` 等证据不足的损耗形(仍保留原文)。

## 怎么实施

1. `recognizer/unit_families.yaml`:10^9/L(11 印形)、10^12/L(4)、μmol/L(5)、
   fL(fl、f1;f1 同形跨 cbc 两族由注册集裁决)、pmol/L、mIU/L;每条注明语料证据
   (cbc_01/02/04/05/06/07/08/09、lft_01/03/04/05/08、tft_01、webwxgetmsgimg_*)。
2. `postprocess.py`:`load_unit_families()`(缓存加载)+ `_normalize_unit(unit, item)`,
   应用于 `run_postprocess` 组装 `ReportItem.unit` 的唯一入口;
   endocrine FT4 units 补登记 `pmol/L`(历史上只登记了损耗形)。
3. golden 重生成(`python -m recognizer.golden`),52 处 unit 变更逐一人工核对
   (全部为表内登记损耗形;RDW-CV 的 `109/L` 等未注册项正确保留)。
4. `internal/storage`(SQL 两处加 `i.low_confidence`)、`internal/pipeline/types.go`、
   `cmd/hdd`(runTrend `--all` + `low_quality` 裁决)。
5. 历史数据:`hdd reparse <sha256...>` 全量重跑后处理(在重新构建
   recognizer 容器使规则生效后;`--force` 重 OCR 非必需,OCR 结果未变)。
   date=null 的单走 `hdd ingest --date` 补录转正;本机 data/ 本地 golden 重跑
   `local_golden.py freeze` 对齐新规则(预计 19/24 不变 + 5 张 unit 收敛)。

## 怎么验收

- pytest 全绿(新增 unit 归一 6 测,含注册集裁决与未注册保留);go test 全绿
  (新增 trend 质量门默认排除/--all 用例)。
- `local_golden score`:matched 230/236=、value 206=、unit 189→193(+4)、ref 194=;
  `gt_eval` samples 指标不变(177/298,160/147/125;单册规范印形与 VL 真值印法
  有 `*`/`×` 差异,得分互有增减,总量不变)。
- 库内验收(部署后):`hdd reparse` 全量后 `./hdd trend 血小板` →
  5 点 4 点可比(delta 生效),`109/L → 10^9/L`、`7/6.0 → 10^9/L`;
  2024 拍照件点被质量门排除(`--all` 可见)。

## 效果指标(改动后)

`local_golden score` matched 230/236、value 206、unit 193、ref 194;
golden report 单位字段与 raw_text 原文留痕并存,人工核对路径不变。
