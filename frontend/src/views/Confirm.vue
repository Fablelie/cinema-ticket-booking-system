<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../store/auth'
import { bookingService } from '../services/api'

const router = useRouter()
const authStore = useAuthStore()

// State สำหรับนับเวลาถอยหลัง
const timeLeftStr = ref('05:00')
let timerInterval = null

// ดึงข้อมูลเก้าอี้ที่เลือกมาจาก Pinia Store ส่วนกลาง
const selectedSeats = computed(() => authStore.selectedSeats)

// 1. ฟังก์ชันคำนวณและอัปเดตเวลานับถอยหลังทุกๆ 1 วินาที (Senior UI Practice)
const startCountdown = () => {
  if (!authStore.lockedUntil) {
    // ป้องกันกรณีผู้ใช้แอบพิมพ์ URL เข้าหน้านี้ตรงๆ โดยไม่มีข้อมูลการจอง
    router.push('/seats')
    return
  }

  const targetTime = new Date(authStore.lockedUntil).getTime()

  timerInterval = setInterval(() => {
    const now = new Date().getTime()
    const difference = targetTime - now

    if (difference <= 0) {
      // Timeout!
      clearInterval(timerInterval)
      timeLeftStr.value = '00:00'
      alert('หมดเวลาทำรายการจองตั๋ว ระบบจะนำคุณกลับไปเลือกที่นั่งใหม่อีกครั้ง')
      
      // ล้างข้อมูลใน Store แล้วเตะกลับหน้าผังที่นั่ง (หลังบ้านจะคอยเคลียร์สเตตัสใน Mongo อยู่แล้ว)
      authStore.selectedSeats = []
      authStore.lockedUntil = null
      router.push('/seats')
      return
    }

    // คำนวณเป็นนาทีและวินาที
    const minutes = Math.floor((difference % (1000 * 60 * 60)) / (1000 * 60))
    const seconds = Math.floor((difference % (1000 * 60)) / 1000)

    // จัดฟอร์แมตให้ออกเป็น 02:05 เสมอ
    const mStr = minutes < 10 ? '0' + minutes : minutes
    const sStr = seconds < 10 ? '0' + seconds : seconds

    timeLeftStr.value = `${mStr}:${sStr}`
  }, 1000)
}

// 2. ฟังก์ชันกดยืนยันการจองตั๋ว (Confirm -> BOOKED ถาวร)
const handleConfirm = async () => {
  try {
    await bookingService.confirmBooking(authStore.defaultShowtimeId, selectedSeats.value)
    alert('ยืนยันการจองตั๋วและชำระเงินสำเร็จเรียบร้อยแล้ว!')
    
    // เคลียร์ค่าเก้าอี้ในสโตร์ และพากลับไปหน้าผังที่นั่งเพื่อดูผลลัพธ์สีเทา
    authStore.selectedSeats = []
    authStore.lockedUntil = null
    router.push('/seats')
  } catch (error) {
    console.error('Confirm error:', error)
    alert('เกิดข้อผิดพลาดในการยืนยันรายการ กรุณาลองใหม่อีกครั้ง')
  }
}

// 3. ฟังก์ชันกดยกเลิกการจองตั๋ว (Cancel -> คืนสถานะกลับเป็น AVAILABLE)
const handleCancel = async () => {
  try {
    // ยิงไปลบล็อกใน Redis และปรับกลับเป็น AVAILABLE ใน MongoDB
    await bookingService.cancelBooking(authStore.defaultShowtimeId, selectedSeats.value)
    
    // เคลียร์ค่าเก้าอี้ในสโตร์ และพากลับไปหน้าผังที่นั่ง
    authStore.selectedSeats = []
    authStore.lockedUntil = null
    router.push('/seats')
  } catch (error) {
    console.error('Cancel error:', error)
    // ถึงแม้เน็ตเวิร์กจะตัดขาด หากกดยกเลิกพลาด พากลับหน้าเดิมเพื่อให้ระบบดึงสถานะจริงมาโชว์ใหม่
    router.push('/seats')
  }
}

onMounted(() => {
  startCountdown()
})

onUnmounted(() => {
  if (timerInterval) {
    clearInterval(timerInterval)
  }
})
</script>

<template>
  <div class="confirm-container">
    <div class="confirm-card">
      <div class="timer-section">
        <span class="clock-icon">⏳</span>
        <p class="timer-label">กรุณาทำรายการภายในเวลา</p>
        <h2 class="timer-countdown" :class="{ 'timer-urgent': timeLeftStr.startsWith('00') }">
          {{ timeLeftStr }}
        </h2>
      </div>

      <div class="details-section">
        <h1 class="title">🎫 ยืนยันรายการจองตั๋ว</h1>
        
        <div class="info-row">
          <span class="label">ภาพยนตร์:</span>
          <span class="value">Take-Home Assignment Cinema</span>
        </div>
        <div class="info-row">
          <span class="label">โรงภาพยนตร์:</span>
          <span class="value">THEATER 1</span>
        </div>
        <div class="info-row">
          <span class="label">ที่นั่งที่คุณเลือก:</span>
          <span class="value seat-tags">
            <span v-for="seat in selectedSeats" :key="seat" class="seat-badge">
              {{ seat }}
            </span>
          </span>
        </div>
      </div>

      <!-- ปุ่มดำเนินการทั้ง 2 ฝั่งตามเงื่อนไขของ Workflow -->
      <div class="action-buttons">
        <button @click="handleConfirm" class="btn-confirm">
          ✅ ยืนยันการจอง (Confirm)
        </button>
        <button @click="handleCancel" class="btn-cancel">
          ❌ ยกเลิกรายการ (Cancel)
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.confirm-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background-color: #0f172a;
  color: #f8fafc;
  font-family: system-ui, -apple-system, sans-serif;
  padding: 20px;
}

.confirm-card {
  background-color: #1e293b;
  border-radius: 16px;
  padding: 40px 30px;
  max-width: 480px;
  width: 100%;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  text-align: center;
}

.timer-section {
  background-color: #0f172a;
  padding: 20px;
  border-radius: 12px;
  margin-bottom: 30px;
  border: 1px solid #334155;
}

.clock-icon { font-size: 32px; display: block; margin-bottom: 8px; }
.timer-label { font-size: 13px; color: #94a3b8; margin: 0 0 5px 0; }
.timer-countdown { font-size: 42px; font-weight: 800; color: #f97316; margin: 0; letter-spacing: 1px; }
/* ถ้าเหลือเวลาต่ำกว่า 1 นาที จะกระพริบเตือนสีแดงเพิ่มความสมจริง */
.timer-urgent { color: #ef4444; animation: pulse 1s infinite; }

.details-section { text-align: left; margin-bottom: 35px; }
.title { text-align: center; font-size: 22px; margin-bottom: 25px; color: #f8fafc; font-weight: 700; }

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #334155;
}
.info-row:last-child { border-bottom: none; }

.label { color: #94a3b8; font-size: 14px; }
.value { font-weight: 600; color: #e2e8f0; font-size: 15px; }

.seat-tags { display: flex; gap: 6px; flex-wrap: wrap; }
.seat-badge {
  background-color: #06b6d4;
  color: white;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 700;
  box-shadow: 0 2px 4px rgba(0,0,0,0.2);
}

.action-buttons { display: flex; flex-direction: column; gap: 12px; }

.btn-confirm {
  background-color: #10b981;
  color: white;
  border: none;
  padding: 14px;
  border-radius: 8px;
  font-weight: 700;
  font-size: 16px;
  cursor: pointer;
  transition: background 0.2s;
}
.btn-confirm:hover { background-color: #059669; }

.btn-cancel {
  background-color: #ef4444;
  color: white;
  border: none;
  padding: 12px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s;
}
.btn-cancel:hover { background-color: #dc2626; }

@keyframes pulse {
  0% { opacity: 1; }
  50% { opacity: 0.6; }
  100% { opacity: 1; }
}
</style>
