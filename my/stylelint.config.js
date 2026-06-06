/** @type {import('stylelint').Config} */
export default {
  extends: [
    'stylelint-config-standard',
    // 让 .vue 的 <style> 块走 postcss-html 解析
    'stylelint-config-standard-vue',
  ],
  rules: {
    // —— Tailwind v4：放行 @theme / @custom-variant / @apply / @variant 等自定义 at-rule ——
    'at-rule-no-unknown': null,
    'import-notation': null,
    // 设计系统里大量自定义类名 / data 属性选择器，不强制 BEM 命名
    'selector-class-pattern': null,
    'custom-property-pattern': null,
    'keyframes-name-pattern': null,
    // 嵌套覆盖较多，关闭降序特异性告警
    'no-descending-specificity': null,
    // oklch / color-mix 等现代颜色写法
    'function-no-unknown': null,
    'alpha-value-notation': null,
    'color-function-notation': null,
    'hue-degree-notation': null,
    // 动效类有意写成单行多声明（紧凑），纯排版规则，关闭
    'declaration-block-single-line-max-declarations': null,
  },
  // .vue 可能没有 <style> 块
  allowEmptyInput: true,
  ignoreFiles: [
    'node_modules/**/*',
    'dist/**/*',
    'dist-ssr/**/*',
    'src/components/ui/**/*',
  ],
}
