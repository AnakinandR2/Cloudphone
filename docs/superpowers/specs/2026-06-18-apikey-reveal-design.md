# API 密钥可重复查看（加密存储 + 显示）— 设计

- 日期：2026-06-18
- 状态：已评审，待实现
- 背景：第一期 API 密钥为「只存 sha256、明文仅创建时显示一次」。本次让密钥可**重复查看**——需把明文以可还原形式存储。

## 1. 目标

让用户随时在密钥列表里「显示」并复制完整密钥，而不必丢失后重建。

## 2. 关键决策（评审锁定）

1. **可还原存储 = 加密存储**（非明文）：AES-256-GCM 加密整密钥存库，查看时解密返回。
2. 鉴权**不变**：仍按不可逆 `key_hash` 比对；加密只为展示。
3. 撤销的密钥仍可「显示」（只读历史），但不能再用于鉴权。
4. `reveal` 不额外要求重输密码——同会话 JWT 即可（用户自己的密钥）。

## 3. 后端

### 3.1 存储

`api_keys` 新增列 `key_cipher`（text）：AES-256-GCM 的 `base64(nonce | ciphertext)`。保留 `key_hash`（鉴权）、`key_prefix`/`key_last4`（掩码展示）。

### 3.2 加密

- 加密密钥来源：配置 `APIKEY_ENC_KEY`（32 字节，hex 或 base64）；未配置时回退 `sha256(JWTSecret + "apikey")`（保证总有一把 32 字节密钥）。
- 算法：AES-256-GCM，每次随机 12 字节 nonce；密文 = `base64(nonce || gcmSeal)`。
- 工具放 `framework/crypto`（通用，供其它模块复用）：`Encrypt(plain string) (string,error)` / `Decrypt(token string) (string,error)`，内部用上面的派生密钥。

### 3.3 接口

- 创建：除哈希外，把明文加密存 `key_cipher`。
- 新增 `GET /api/v1/user/api-keys/:id/reveal`（JWT + 属主校验）→ 解密 `key_cipher` 返回 `{fullKey}`。
- 旧密钥兼容：`key_cipher` 为空 → 422「该密钥不支持显示，请重建」。

## 4. 前端（my）

- 密钥表「密钥」列加「显示」眼睛图标操作：点击 → 调 reveal → 弹出完整密钥 + 复制（可重复）。撤销密钥也可显示。
- 创建后的一次性展示对话框保留（即时复制方便），文案去掉「关闭后无法再查看」，改为「也可随时在列表点『显示』查看」。
- 复用现有「完整密钥」对话框组件展示 reveal 结果。
- `api/modules/apikey.ts` 加 `reveal(id)`；mock 补 reveal；i18n 补 `reveal/revealFail/legacyNoCipher` 等键（中英）。

## 5. 安全说明

- 加密仅供展示便利，鉴权仍走哈希。
- DB 泄露需同时拿到 `APIKEY_ENC_KEY` 才能还原明文。
- **生产务必显式配置 `APIKEY_ENC_KEY`**；否则随 `JWTSecret` 派生，改 `JWTSecret` 会导致旧密文不可解（reveal 失败，但鉴权与撤销不受影响）。

## 6. 测试

- `framework/crypto`：`Encrypt` → `Decrypt` 往返一致；篡改密文解密失败。
- openapi：create 后 reveal 返回与创建响应一致的明文；无 cipher 的旧密钥 reveal 报错；越权 reveal（非属主）→ 404；撤销密钥仍可 reveal。
- 前端 `pnpm build` 过；mock 跑通显示。

## 7. 不在本期

- reveal 审计日志 / 频率限制。
- 加密密钥轮换（rotation）。
