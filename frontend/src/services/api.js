import axios from 'axios'

// 1. ตั้งค่าพื้นฐานสำหรับเรียกใช้ API หลังบ้าน Go-Gin
const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 2. แปะ Google Token เข้าไปใน Header อัตโนมัติก่อนส่งคำขอออกไป
apiClient.interceptors.request.use((config) => {
  // ดึง Token ที่ได้จาก Google Sign-In ซึ่งเราจะเก็บไว้ใน localStorage ของเบราว์เซอร์
  const token = localStorage.getItem('google_id_token')
  
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
}, (error) => {
  return Promise.reject(error)
})

// 3. รวมฟังก์ชันสำหรับเรียกใช้ตาม Workflow ของโจทย์
export const seatService = {
  // ดึงผังที่นั่งรอบแรกตอนเปิดหน้าจอ p_seat
  getSeatMap(showtimeId) {
    return apiClient.get(`/seats/${showtimeId}`)
  }
}

export const bookingService = {
  // กด Next จากหน้า p_seat เพื่อขอล็อกที่นั่ง (All-or-Nothing)
  reserveSeats(showtimeId, seats) {
    return apiClient.post('/bookings/reserve', { showtime_id: showtimeId, seats })
  },
  // กด Confirm จากหน้า p_confirm เพื่อเปลี่ยนสถานะเป็น BOOKED ถาวร
  confirmBooking(showtimeId, seats) {
    return apiClient.post('/bookings/confirm', { showtime_id: showtimeId, seats })
  },
  // กด Cancel จากหน้า p_confirm เพื่อปล่อยเก้าอี้กลับเป็น AVAILABLE ทันที
  cancelBooking(showtimeId, seats) {
    return apiClient.post('/bookings/cancel', { showtime_id: showtimeId, seats })
  }
}

export const adminService = {
  // แอดมินเรียกดูข้อมูลประวัติกิจกรรมพร้อมระบบ Filter บน Dashboard
  getDashboardData(params) {
    return apiClient.get('/admin/dashboard', { params })
  }
}

export default apiClient
