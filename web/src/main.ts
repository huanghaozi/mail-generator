import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import router from './router'
import { createPinia } from 'pinia'
import Antd from 'ant-design-vue'
import 'ant-design-vue/dist/reset.css'
import i18n from './locales'

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(router)
app.use(Antd)
app.use(i18n)
app.mount('#app')
