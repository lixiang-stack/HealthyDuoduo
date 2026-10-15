# 04: 工程骨架级纵切：App↔后端第一根管

**What to build:** 手机上的 App 第一次实时"看见"家庭后端：冷启动显示后端连通状态与版本；后端地址可在设置里手输并持久化保存。这是二维码配对（11 票）之前的兜底路径，并把客户端的全部地基立起来。

**Blocked by:** 01（`hdd serve` 骨架落真栈）, 02（M0 契约冻结）。

**Status:** resolved

- [x] 单一前端工程就位（React + Expo + TypeScript）：API 封装由 OpenAPI codegen 生成；验证环境用 Expo Go/模拟器
- [x] platform 抽象模块骨架：验证环境（文件选择）与真机（相机/相册）的差异收口点建立，页面不散落原生分支
- [x] App 冷启显示后端连通状态与版本（打 health），未连通呈现可理解的失败态而非崩溃
- [x] 设置页手输 Base URL 持久化，连通校验走 health；真机联调不误用不可达的 localhost
- [x] 纯逻辑单测 seam 建立并落首个用例（连通状态判定），全绿

## Comments

**实现（分支 `mobile-scaffold-health`，worktree `.worktree/mobile-scaffold-health`，单 commit，push/PR 待用户）**

- **工程**：`apps/mobile/`（Expo SDK 57 / RN 0.86 / React 19.2 / TS 6.0，expo-router 文件路由）。`create-expo-app` blank-typescript 模板打底，清掉模板残留（`App.tsx`/`index.ts`/`LICENSE`/`.claude/`），`main` 改 `expo-router/entry`，app.json 加 `scheme`/`web.bundler`。
- **codegen 选型（先 research 再定，handoff 要求）**：**openapi-typescript 7.13 + openapi-fetch 0.17**。理由：3.1 原生（契约里用了 `oneOf`/`const`/`$ref`，已用真实 `schemas/openapi.yaml` 实测生成成功）；openapi-generator 的 TS target 对 3.1 尤其 `oneOf` 支持弱；Hey API 生成面更大、此票不需要。`npm run codegen` 写 `src/api/schema.d.ts`（提交入库，CI 校验与 `openapi.yaml` 同步 = M0「生成物同步」判据）。
- **API 层**：`src/api/schema.d.ts`(生成) / `client.ts`(`createApiClient`) / `connectivity.ts`(**纯逻辑 seam**：`resolveConnectivity(probe)` 判定 ok / unreachable / error) / `health.ts`(`probeHealth`+`checkHealth`，把 openapi-fetch 的网络异常/非 2xx/成功收敛为 `HealthProbe`)。
- **platform 抽象**：`src/platform/imagePicker.ts` 是唯一出现 `Platform.OS` 分支与 expo-image-picker 的地方；`pickImage('camera'|'library')` 对 web 相机收口为 `unsupported`（验证环境用文件选择）。业务页面只调 `pickImage`（本票尚未消费，上传票 06 接）。
- **页面**：`src/app/{_layout,index,settings}.tsx`（冷启动首页打 health 显示版本；设置页手输 + 保存 + 校验）；`src/components/ConnectivityStatus.tsx` 两页共用。Base URL 纯逻辑 `src/settings/baseUrl.ts`（`deriveDefaultBaseUrl`/`normalizeBaseUrl`）+ 持久化 `baseUrlStore.ts`(AsyncStorage)。
- **真机不误用 localhost**：默认地址从 Expo 开发服务器 host(`Constants.expoConfig.hostUri`)推导 `http://<LAN-IP>:8080/api/v1`；已存值优先；两者皆无 → `unconfigured` 态引导去设置。

**Decision / 有意偏离**
1. **页面文件即路由**：design doc 建议 `src/screens/`，本票让屏幕直接住在 `src/app/` 路由文件里——expo-router 规定路由即文件，再包一层 `screens/` 只产生纯转发文件（AGENTS.md 禁止「只为转发而存在的模块」）。`src/api`/`src/platform`/`src/settings` 等共享层照旧。
2. **保留模板 `apps/mobile/AGENTS.md`**：Expo 官方给子工程的 agent 指引（不信任训练数据、用 `expo install` 等），对本仓后续 agent 有用，故保留。
3. **`.npmrc: legacy-peer-deps=true`**：openapi-typescript 的 peer 仍 `typescript ^5.x`（Expo 模板给 TS 6），属已知 peer 范围滞后而非真实不兼容；记入文件使本地与 CI 的 `npm ci` 解析一致。jest/@types/jest 按 expo-doctor 期望落到 29.7/29.5.14。
4. **门禁接入**：CI 新增 `mobile` job（`setup-node@v7`/node 22/`npm ci`/codegen 同步 diff/typecheck/test）；lefthook 增 `mobile-typecheck`/`mobile-test`/`mobile-codegen-sync`，与 CI 双链一致。

**验证证据**
- `npm run typecheck` 空；`npm test` 2 套件 8 用例全绿（connectivity 3：ok/http/network；baseUrl 5：LAN 推导、无 host、去空白末尾斜杠、补 API 前缀、拒非法协议）。
- `npx expo-doctor` 21/21；`npx expo export` web 与 ios 均打包成功（日志确认 `src/app` 为 router 根）。
- **真实后端端到端**（共享 deploy 栈 `hdd serve` 在跑）：tsx 直跑 `checkHealth` → `127.0.0.1:8080/api/v1` = `{status:"ok",version:"0.1.0"}`（版本与 live 一致）；死端口 = `{status:"unreachable",detail:"fetch failed"}`；黑洞地址（`10.255.255.1`）= `{status:"unreachable",detail:"连接超时"}`（5s 超时生效）——证明 codegen 类型 + openapi-fetch + 判定层在真栈上成立，且三类失败态都不崩。
- Go 侧零回归：`go vet ./...` 空、`go test ./...` 全绿（node_modules 不影响 Go 工具链）。

**code-review 修正（两轴并行评审后并入本 commit）**
- 🔴 修复（Standards·魔法值）：`'#6b7280'` 等色值跨 3 文件重复——抽 `src/components/theme.ts` 的 `colors` 一处定义。
- 🔴 修复（Spec·失败态）：`probeHealth` 原样裸 fetch 无超时，黑洞地址会让首页永远停在「正在检测」，够不上验收 3「呈现可理解的失败态」——加 5s `AbortController` 超时，超时归 `unreachable`（detail「连接超时」）。
- 修复（Spec·易用性）：`normalizeBaseUrl` 原样接受 `http://host:8080`（缺 `/api/v1`）→ 存下后 health 打 404 报「响应异常」误导；改为缺前缀时补 `API_PREFIX`（家长多直接输 IP）。
- 有意保留：(1) 首页/设置页「加载→推导→探活」4 行重复——两页流程本就不同（设置页另有输入框与保存后重探），为一个 hook 造跨域耦合不值，judgement call；(2) `pickImage` 无消费者——票面要求先立 seam，judgement call；(3) `.npmrc legacy-peer-deps` 面偏大但已披露且被 openapi-typescript 的滞后 peer 逼出——保持；(4) CI/lefthook 同步命令两处重写 = 本仓「双链」惯例，用户本轮明确选择本次接入。

**披露 / 留给下票**
- **Expo Go / 模拟器交互运行未在本会话执行**：本机无 `simctl`（无 Xcode 命令行工具）也无 Expo Go，故真机/模拟器点检留待维护者；机器侧证据为 bundle export（web+ios）+ expo-doctor + 真栈 API 探活。**建议合前用 Expo Go 走一遍冷启动连通 + 设置页保存**。
- 真机默认地址依赖 `Constants.expoConfig.hostUri`（仅 dev 有）；生产构建下为 undefined → 落 `unconfigured`，配对二维码（票 11）补位。
- `pickImage` 已实现但无消费者（上传票 06 接入）；本票只建 seam。
- 环境小坑（沿用 handoff）：`hdd` 栈 `--from` 传绝对路径；本票未动共享库，无需 snapshot/refresh。

