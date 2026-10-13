# 01: `hdd serve` 骨架落真栈

**What to build:** 家长 App 的后端入口第一次存在：既有 CLI 二进制新增 serve 子命令成为常驻 HTTP 服务，随既有 compose 栈一起拉起，局域网上任何客户端都能探到它的心跳并读到版本。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] `hdd serve`（带监听地址/端口参数）启动后，真栈上 `GET /api/v1/health` 返回 `{ok, version}`
- [ ] compose 一条命令拉起四件套（postgres / 对象存储 / 识别服务 / 数据服务），数据服务对识别服务的依赖经既有依赖注入层就位
- [ ] OpenAPI 3.1 契约文件初始化：`/api/v1` 前缀 + health 端点冻结入库
- [ ] HTTP 边界集成测试骨架落位（复用既有编排依赖 fakes 模式），health 用例绿
- [ ] 按 e2e 文档护栏对共享栈冒烟（快照锚点习惯保持）
