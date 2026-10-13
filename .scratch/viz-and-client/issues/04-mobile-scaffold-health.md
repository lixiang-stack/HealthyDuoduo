# 04: 工程骨架级纵切：App↔后端第一根管

**What to build:** 手机上的 App 第一次实时"看见"家庭后端：冷启动显示后端连通状态与版本；后端地址可在设置里手输并持久化保存。这是二维码配对（11 票）之前的兜底路径，并把客户端的全部地基立起来。

**Blocked by:** 01（`hdd serve` 骨架落真栈）, 02（M0 契约冻结）。

**Status:** ready-for-agent

- [ ] 单一前端工程就位（React + Expo + TypeScript）：API 封装由 OpenAPI codegen 生成；验证环境用 Expo Go/模拟器
- [ ] platform 抽象模块骨架：验证环境（文件选择）与真机（相机/相册）的差异收口点建立，页面不散落原生分支
- [ ] App 冷启显示后端连通状态与版本（打 health），未连通呈现可理解的失败态而非崩溃
- [ ] 设置页手输 Base URL 持久化，连通校验走 health；真机联调不误用不可达的 localhost
- [ ] 纯逻辑单测 seam 建立并落首个用例（连通状态判定），全绿
