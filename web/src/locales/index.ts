import { createI18n } from 'vue-i18n'
import en from './en'
import zh from './zh'

const i18n = createI18n({
  legacy: false, // Use Composition API
  locale: localStorage.getItem('locale') || 'zh', // Default to Chinese
  fallbackLocale: 'en',
  messages: {
    en,
    zh
  }
})

export default i18n

