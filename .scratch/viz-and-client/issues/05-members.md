# 05: 成员（B1）纵切

**What to build:** 家长在 App 里几十秒配好全家档案：给每个成员一个名字、标记是否孩子；孩子永远排在列表最前。这是所有后续归属、过滤、范围决策的地基。

**Blocked by:** 02（M0 契约冻结）, 04（工程骨架级纵切）。

**Status:** resolved

- [x] App 建成员（显示名 + 是否孩子）、改名、改是否孩子；无删除入口（文案/交互不暗示可删）
- [x] 成员列表孩子优先、其余按创建序（客户端排序）；空态引导创建
- [x] 选中成员的上下文为后续票备好（时间线过滤、上传归属默认当前成员）
- [x] API 集成测试（httptest + 编排 fakes）：建/列/改、存量无主报告与成员独立存在、排序口径
- [x] fixtures 对账绿

## Comments

**实现（分支 `members`，worktree `.worktree/members`，单 commit，push/PR 待用户）**

- **Go API**：`cmd/hdd/serve_members.go` 三端点（GET/POST `/api/v1/members`、PATCH `/members/{member_id}`）。严格解码：未知字段、显式 `null`（`optField` 区分「未带/显式 null/有值」）、尾随数据、类型不符一律 400——`required`（建）与 `minProperties:1`（改）按契约端侧执行；未知成员 404、存储故障 500，全部走 `httpapi.Error` 形状。请求体出口收敛为 `memberBody.create()/patch()`（传输校验与领域取值各收口一处，handler 只做解码→取值→调用→映射错误）；成员名非空白规则上收 `pipeline.ValidateMemberName`（哨兵 `ErrMemberNameBlank`），storage 写入边界镜像复校验（票 03「边界保证 + 入库镜像」双保险惯例）。JSON 收口助手（`writeJSON`/`writeErrorJSON`/`decodeJSONBody`）上移到 `serve.go`，health 同用。
- **边界与存储**：新增 `pipeline.MemberStore`（独立于 `Store` 的窄接口，ISP——不让 CLI 侧既有 fake 承担成员方法；serve 经 `deps.members` 注入同一 `*storage.Store`）；`storage` 三个常量 SQL：建用 `RETURNING`、列表 `ORDER BY id`、PATCH 用 `COALESCE` 只覆盖传入字段。
- **排序口径（两端分工）**：服务端只保证创建序（id 升序，孩子不前置）；孩子优先由客户端 `src/members/order.ts` 做。客户端排序按 `member_id` 而非 `created_at`：BIGSERIAL 不重用，id 序即创建序；`created_at` 为秒级精度，本次真栈实测两成员同秒创建（见证据），id 更精确。
- **App**：`src/app/members.tsx` 成员页（空态引导建；列表孩子优先；点选 = 当前成员，选中行标「当前成员」；行内编辑改名/改档；无删除入口）；`src/members/components.tsx` 展示组件；`src/api/members.ts` 封装（错误体 `message` 透传）；`src/members/CurrentMemberContext.tsx` + `currentMemberStore.ts` 当前成员上下文（AsyncStorage 持久化，票 06+ 消费）；`src/api/resolveClient.ts` 三页共用客户端解析（消 index/settings/members 三处重复）；`_layout` 注册路由 + 首页入口链接。
- **测试证据**：httptest 集成测试覆盖 CRUD 全回路、校验矩阵（缺字段/空名/空白名/null/错类型/未知字段/畸形 JSON/非对象/空体，拒绝后零成员落库）、404/400、空信封 `{"members":[]}`；`pipeline/members_test.go` 领域规则单测（含全角空格边界）；`storage/store_member_test.go` 真库 gated 冒烟（一次性 PG：COALESCE 部分更新、created_at 保持、缺失 `nil,nil`、空白名镜像拒绝、存量无主报告不动）。`go test ./...` 全绿（含 fixtures 对账）；mobile jest 4 套件 16 用例（新增排序 5 + 当前成员持久化 3）、typecheck、codegen 同步 diff、`expo export` web 打包成功。
- **共享栈真栈 e2e**：snapshot `data/e2e-snapshots/pre-members-20261015-192644.dump`（82308B）→ 按本 worktree 重建 hdd 容器（goose 4 无新迁移）→ curl 建/列/改（含仅 name、仅 is_child、显式 false）、null 拒 400、DELETE 405、未知成员 404 → SQL 核对 **25 份存量报告全部 `member_id IS NULL` 未被触碰**（存量无主报告与成员独立存在）→ mobile 封装以 tsx 直跑真栈五路径（建/改/列/400 错误体/网络失败）→ 冒烟成员 SQL 清理，库回 0 成员。

**Decision / 有意偏离**

1. **当前成员选择显式点按**，不自动选中首成员/新建成员：票面只要求「上下文备好」，上传归属的「选择/确认」是票 06 的交互，默认值由它决定。选择持久化（重载仍在），成员不可删除故已存 id 恒有效。
2. **排序按 id 不按 created_at**：见上；`created_at` 仍按契约返回（展示用），排序注释写明依据。
3. **UI 交互不写单测**（spec 口径：由验证环境门禁覆盖）；纯逻辑（排序、持久化解析）有单测。
4. **`memberBody` 不复用 `httpapi.MemberUpdate`**：契约镜像的指针字段无法区分「未带」与「显式 null」，`optField` 语义不同（更多细节见 Go 测试矩阵）。

**披露 / 留给维护者**

- **浏览器交互验收未跑**（本机无浏览器操作通道）：建议点检 空态→建（孩子/成人各一）→改名→改档→点选「当前成员」→ 重载后选中仍在 → 列表孩子优先。Web 通道现可用。
- **共享栈现状**：deploy 栈 hdd = 本分支代码 + `HDD_SERVE_CORS_ORIGIN='*'`（浏览器验收窗口，与上一会话同姿势）；合入后建议按 main 重建并把该变量留缺省关（ADR-0006）。冒烟数据已自行清理；维护者浏览器验收创建的 5 名成员保留未动，25 份存量报告仍全部无主。
- **serve HTTP 冒烟尚是手工 curl 序列**：spec「Go 改动」档提到的 e2e 工具扩展（`tools/e2e.py` 加 serve 冒烟命令）本票未做，可留后续票。

**评审问答结论（2026-10-16 讨论后记录，代码无改动）**

1. **`deps` 的 `store`/`members` 双字段保留**：同一 `*storage.Store` 注入两个窄端口（成员域 / 报告域持久化端口分离，ISP），既有 CLI/pipeline fakes 零膨胀；合并为单接口需重写既有 fake，收益不成立。属 judgement call，日后要收拢可合并。
2. **三层成员类型保留**：`pipeline.Member`（领域视图） / `httpapi.Member`（契约镜像） / `memberBody`（解码三态）职责不同——合并任一都会破坏依赖方向（内层感知线格式）或 null 严格性。DTO 在契约镜像包、映射（`toAPIMember`）在适配器；`httpapi.MemberCreate/MemberUpdate` 目前零消费者、零 fixture 锚（票 02 产物），留待票 06/09 运行时校验决策时一并处理。
3. **storage 镜像复校验保留**（票 03「边界保证 + 入库镜像」惯例）：失败即调用方绕过边界校验的 bug 信号；升级路径为条件触发——出现第二写入路径或第二条名字规则时改为值对象（构造即校验），镜像检查随之消失。
4. **`optField` 三态语义保留**：PATCH 中「缺失」是合法语义（别碰该字段）、「显式 null」是非法值，两者在普通指针下解出同一结果（nil）——「发现 nil 即 400」会误杀合法部分更新（如仅改 `is_child`），必须在解码期区分；POST 上两者同 400、仅文案不同。
5. **手写校验暂不引 validate 库**：未知字段/尾随数据/三态本就是 decoder 层行为，struct-tag 类库替代不了；重访点 = 票 09 `ReviewRequest` 的 `oneOf`（条件式跨字段校验出现时）。
