# cloudphone-tester ↔ main-monorepo · 缺陷流转协议说明书（qa/ 契约·第5协议右环）

> 本文 = **协议契约说明书**（不是缺陷数据·缺陷 100% 走 GitLab issue）。目的：让本仓研发**感知**——
> cloudphone-tester（独立黑盒测试方）会给你什么样的缺陷 issue、需要你回什么样的修复回执。
> **权威源（SoT）**：cloudphone-tester 仓 `qa-suites/main/qa-protocol.md`；本文件经 `qa` 隔离分支同步至此。
> 缺陷记录字段沿用本仓既有缺陷规范 `docs/测试/测试计划.md`（复现·环境·期望·实际·证据·用例ID），不另造格式。
> **严重度分级映射**：我方 issue 用 **S0–S3**（我方规范·已重编号）；对应本仓《测试计划》§8 的 **S1–S4** 为 **−1 偏移**——我方 `S0/S1/S2/S3` = 你们 `S1/S2/S3/S4`（即我方 **S0/S1 = 你们准出门的 S1/S2** 阻断/严重）。优先级 `P0–P2` 两方一致。

## 1. 我方 → 你：缺陷 issue 长什么样
- **建在**：本仓 GitLab issue（`source::cloudphone-tester` + `defect` + `S?`/`P?` + 模块名 label）。
- **标题**：`[CP-xxxx] 一句话问题`（模块见正文）。
- **正文**：严重度/优先级 · 模块 · 被测版本(commit sha) · 复现率 · 复现步骤 · 根因(file:line 线索) · 影响。
- **已脱敏**：所有 issue 内容过我方确定性脱敏闸（去中台地址/内网IP/AKSK/token/真机标识），**绝不含敏感信息**。
- **机读锚点**：正文尾部含 ```yaml defect-card``` 块（id/issue_repo/severity/version·供我方回程对账·勿删）。
- 🔶 **`suspect-external` label（疑外部·归口确认）**：根因疑在**外部云手机中台**（非本仓源码）的缺陷，我方按"消费方"归口到本仓，**请你作为集成方确认归属**——是「我方集成/透传 bug」（修之）还是「纯外部中台 bug」（回执填 `verdict=external`，我方据此转中台方）。**不是要你修中台**，是请你判这一刀。issue 正文顶部有醒目提示。

## 2. 你 → 我方：修复后请回什么（fix-receipt）
修复后请 **close 该 issue**，并在评论里**保留并填写**正文给出的回执块（贴合你们 Conventional Commits 习惯，
建议 commit 用 `fix(模块): …` + `Closes #<iid>`）：

```yaml fix-receipt
fixed_in: ""          # 🔴 必填·修复 commit sha 或版本 tag —— 驱动我方"复测版本≥fixed_in"版本闸
verdict: fixed         # fixed | external | not-a-bug
fix_summary: ""        # 修复说明
impact_scope: ""       # 影响面/改了哪些
dev_selftest: ""       # 你的自测结果
related_mr: ""         # 可选
```
- `fixed_in` 本仓现**无 tag** → 填**修复 commit sha** 即可（后续若引入语义化 tag 再升级）。
- 我方收到 `verdict=fixed` **不直接判"验证成功"**——会用 ≥`fixed_in` 的版本**真复测**通过才判验证成功（防假阳）；不通过则判**验证失败 + reopen 本单 + 追评失败证据**（同单·非新单）。
- `verdict=external`（属中台/上游）/ `verdict=not-a-bug`（不予修复）→ 我方据此转外部/待确认。
- 没填回执只 close + 关联 MR/commit 也行（我方人工兜底推断 `fixed_in`），但**填回执最省事**。

## 3. 边界（严格管控·避免干扰你们的工作）
- 我方对本仓的提交**只动 `qa/**` 路径**（pre-push 脚本硬校验），**绝不碰 main 主干、绝不改源码**。
- 协议文档走常驻隔离分支 `qa`（Developer 直推·不提 MR·不打扰主干），你可随时无视/删该分支。
- qa/ 内**不放可执行脚本**，不触发 CI。
- 本协议**只以本仓 `qa/` 路径文档形式提供**（`qa` 隔离分支·dev clone 即见），**不计入 GitLab issue**——issue 池只装缺陷，协议不占 issue（避免污染缺陷度量/语义错位）。

— 维护方 cloudphone-tester（全局 tester）· 详见我方 `design/右环-缺陷流转qa交换协议-落地方案.md`
