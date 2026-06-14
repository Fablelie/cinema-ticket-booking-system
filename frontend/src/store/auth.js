import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import apiClient from '../services/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('google_id_token') || null)
  const email = ref(localStorage.getItem('user_email') || '')
  const userId = ref(localStorage.getItem('user_id') || '')
  const userRole = ref(localStorage.getItem('user_role') || 'USER')

  const selectedSeats = ref(JSON.parse(localStorage.getItem('held_seats')) || [])
  const lockedUntil = ref(localStorage.getItem('held_until') || null)
  const defaultShowtimeId = ref(import.meta.env.VITE_DEFAULT_SHOWTIME_ID)

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => userRole.value === 'ADMIN')

  const fetchUserProfile = async () => {
    try {
      const response = await apiClient.get('/bookings/me')
      userRole.value = response.data.role
      localStorage.setItem('user_role', response.data.role)
      return response.data.role
    } catch (error) {
      console.error('[STORE ERROR] Failed to fetch server role:', error)
      logout()
      throw error
    }
  }

  const setLoginSession = (googleIdToken, userEmail, googleUserId) => {
    token.value = googleIdToken
    email.value = userEmail
    userId.value = googleUserId
    localStorage.setItem('google_id_token', googleIdToken)
    localStorage.setItem('user_email', userEmail)
    localStorage.setItem('user_id', googleUserId)
  }

  const logout = () => {
    token.value = null
    email.value = ''
    userId.value = ''
    userRole.value = 'USER'
    clearHoldingSeats()
    localStorage.removeItem('google_id_token')
    localStorage.removeItem('user_email')
    localStorage.removeItem('user_id')
    localStorage.removeItem('user_role')
  }

  const saveHoldingSeats = (seatsArr, untilTime) => {
    selectedSeats.value = seatsArr
    lockedUntil.value = untilTime
    localStorage.setItem('held_seats', JSON.stringify(seatsArr))
    localStorage.setItem('held_until', untilTime)
  }

  const clearHoldingSeats = () => {
    selectedSeats.value = []
    lockedUntil.value = null
    localStorage.removeItem('held_seats')
    localStorage.removeItem('held_until')
  }

  return {
    token,
    email,
    userId,
    userRole,
    selectedSeats,
    lockedUntil,
    defaultShowtimeId,
    isAuthenticated,
    isAdmin,
    setLoginSession,
    fetchUserProfile,
    logout,
    saveHoldingSeats,
    clearHoldingSeats
  }
})