import { createApp } from 'vue'
import { createPinia } from 'pinia'
import GoogleSignInPlugin from 'vue3-google-signin'

import App from './App.vue'
import router from './router'

const app = createApp(App)

// 1. ติดตั้งระบบจัดการคลังตัวแปรส่วนกลาง (Pinia)
app.use(createPinia())

// 2. ติดตั้งระบบจัดเส้นทางเดินหน้าจอเว็บ (Vue Router)
app.use(router)

// 3. 🔐 ผูกฐานปุ่ม Google OAuth 2.0 เข้ากับรหัส Client ID จาก .env
const googleClientId = import.meta.env.VITE_GOOGLE_CLIENT_ID || 'your_google_client_id'
app.use(GoogleSignInPlugin, {
  clientId: googleClientId,
})

app.mount('#app')
