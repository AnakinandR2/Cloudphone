import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { i18n } from './locales'

import router from './router'
import { useSettingsStore } from './stores/settings'
// 本地字体（Plus Jakarta Sans 正文 / Fira Code 等宽），避免运行时依赖 Google Fonts
import '@fontsource/plus-jakarta-sans/400.css'
import '@fontsource/plus-jakarta-sans/500.css'
import '@fontsource/plus-jakarta-sans/600.css'
import '@fontsource/plus-jakarta-sans/700.css'

import '@fontsource/fira-code/400.css'
import '@fontsource/fira-code/500.css'
import './assets/index.css'
import './assets/auth.css'

const app = createApp(App)

app.use(createPinia())
app.use(i18n)
app.use(router)

// 启动时把设置（明暗/主题色等）应用到 <html>
useSettingsStore().apply()

app.mount('#app')
