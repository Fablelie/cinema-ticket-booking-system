import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('google_id_token') || null)
  const email = ref(localStorage.getItem('user_email') || '')
  
  const selectedSeats = ref([])
  const lockedUntil = ref(null)

  // ✅ แก้ไขจุดที่ 1: ดึง Default Showtime ID จาก .env ผ่าน Vite Environment
  const defaultShowtimeId = ref(import.meta.env.VITE_DEFAULT_SHOWTIME_ID)

  const isAuthenticated = computed(() => !!token.value)
  
  // ✅ แก้ไขจุดที่ 2: ดึงรายชื่ออีเมลแอดมินจาก .env แล้วนำมาหั่นเช็กสิทธิ์แบบไดนามิก
  const isAdmin = computed(() => {
    if (!email.value) return false

    const adminEmailsEnv = import.meta.env.VITE_ADMIN_EMAILS || ''
    const adminList = adminEmailsEnv.split(',').map(e => e.trim())

    // เช็กว่าอีเมลเราตรงกับในรายชื่อ หรือลงท้ายด้วยโดเมนแอดมินองค์กรหรือไม่
    return adminList.includes(email.value)
  })

  const setLoginSession = (googleIdToken, userEmail) => {
    token.value = googleIdToken
    email.value = userEmail
    localStorage.setItem('google_id_token', googleIdToken)
    localStorage.setItem('user_email', userEmail)
  }

  const logout = () => {
    token.value = null
    email.value = ''
    selectedSeats.value = []
    lockedUntil.value = null
    localStorage.removeItem('google_id_token')
    localStorage.removeItem('user_email')
  }

  return {
    token,
    email,
    selectedSeats,
    lockedUntil,
    defaultShowtimeId,
    isAuthenticated,
    isAdmin,
    setLoginSession,
    logout
  }
})
