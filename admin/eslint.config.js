import antfu from '@antfu/eslint-config'

// 基于 @antfu/eslint-config（Vue + TS + 内置 stylistic 格式化，单引号 / 无分号 / 2 空格，
// 与现有代码风格一致）。只在其基础上做少量"对齐本项目写法 + 保留易错规则"的调整。
export default antfu(
  {
    type: 'app',
    vue: true,
    typescript: true,
    // 项目用 Tailwind，无需 markdown / yaml lint，聚焦 ts/vue/json
    markdown: false,
    yaml: false,
    ignores: [
      'dist',
      'dist-ssr',
      'public',
      // shadcn-vue 生成的组件，保持上游风格，不纳入规范
      'src/components/ui/**',
    ],
  },
  {
    rules: {
      // —— 易引发问题、保留为 error ——
      // console 仅允许 warn/error（业务日志走 toast，调试 log 不应进仓库）
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      // 允许短路 / 三元表达式语句（项目里有 `cond && fn()` 写法）
      'ts/no-unused-expressions': ['error', {
        allowShortCircuit: true,
        allowTernary: true,
      }],

      // —— 对齐本项目写法、降噪 ——
      // 不强制顶层函数声明式写法（项目用 const fn = () => {} 与 function 混用）
      'antfu/top-level-function': 'off',
      // 不强制 if 单行换行
      'antfu/if-newline': 'off',
    },
  },
  {
    files: ['src/**/*.vue'],
    rules: {
      // SFC 块顺序与项目一致：script → template → style
      'vue/block-order': ['error', { order: ['script', 'template', 'style'] }],
    },
  },
)
