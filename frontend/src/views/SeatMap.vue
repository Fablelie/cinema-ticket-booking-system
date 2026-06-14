<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../store/auth'
import { seatService, bookingService } from '../services/api'
import { useWebSocket } from '../services/websocket'

const router = useRouter()
const authStore = useAuthStore()
const { connect, disconnect, latestEvent } = useWebSocket()

// State ภายในคอมโพเนนต์
const seats = ref([]) // เก็บผังเก้าอี้ทั้งหมดที่ดึงมาจาก Server
const localSelected = ref([]) // 🔵 Local State สำหรับเก็บเก้าอี้ที่เราคลิกเลือก (สีฟ้า)
const isLoading = ref(true)
const errorMessage = ref('')

const isHoldingActiveLock = ref(false)
const timeLeftStr = ref('')
let localTimerInterval = null

// แยกกลุ่มเก้าอี้เป็น แถว A และ แถว B เพื่อให้วนลูป Render ง่ายขึ้น
const rowA = computed(() => seats.value.filter(s => s.seat_no.startsWith('A')))
const rowB = computed(() => seats.value.filter(s => s.seat_no.startsWith('B')))

// 1. ดึงข้อมูลผังที่นั่งรอบแรกเมื่อเปิดหน้าจอเข้ามา
const fetchInitialSeatMap = async () => {
  try {
    isLoading.value = true
    const response = await seatService.getSeatMap(authStore.defaultShowtimeId)
    seats.value = response.data.seats
  } catch (error) {
    console.error('Failed to load seat map:', error)
    errorMessage.value = 'ไม่สามารถโหลดผังที่นั่งได้ กรุณาลองใหม่อีกครั้ง'
  } finally {
    isLoading.value = false
  }
}

const startLocalTime = () => {
  if (localTimerInterval) clearInterval(localTimerInterval)
  if (!authStore.lockedUntil) {
    isHoldingActiveLock.value = false
    return
  }

  const targetTime = new Date(authStore.lockedUntil).getTime()

  localTimerInterval = setInterval(() => {
    const now = new Date().getTime()
    const difference = targetTime - now

    if (difference <= 0) {
      // เมื่อครบกำหนด 5 นาที หลุดจอง! ดีดหน้าจอของเขากลับมาอยู่ในสถานะปกติ
      clearInterval(localTimerInterval)
      isHoldingActiveLock.value = false
      authStore.clearHoldingSeats() // ล้างค่าใน LocalStorage
      localSelected.value = []      // ล้างปุ่มสีฟ้า
      fetchInitialSeatMap()         // โหลดผังที่นั่งสีแดงล่าสุดกลับมาแสดงผล
      return
    }

    isHoldingActiveLock.value = true
    
    // แปลงวินาทีโชว์ข้อความแจ้งเตือนด้านล่าง
    const minutes = Math.floor((difference % (1000 * 60 * 60)) / (1000 * 60))
    const seconds = Math.floor((difference % (1000 * 60)) / 1000)
    timeLeftStr.value = `${minutes < 10 ? '0' + minutes : minutes}:${seconds < 10 ? '0' + seconds : seconds}`
  }, 1000)
}

// 2. ฟังก์ชันจัดการเมื่อเราคลิกเลือกเก้าอี้ (เปลี่ยนสถานะ Local เป็น สีฟ้า)
const toggleSeatSelection = (seatNo, status) => {
  if (isHoldingActiveLock.value) return

  // กดได้เฉพาะเก้าอี้ที่เป็น AVAILABLE (สีแดง) เท่านั้น
  if (status !== 'AVAILABLE') return

  if (localSelected.value.includes(seatNo)) {
    // ถ้าเคยเลือกอยู่แล้ว ให้คลิกอีกรอบเพื่อถอนการเลือก
    localSelected.value = localSelected.value.filter(s => s !== seatNo)
  } else {
    // เพิ่มเก้าอี้เข้าไปในกล่องรายการเลือกในเครื่องของเรา
    localSelected.value.push(seatNo)
  }
}

// 3. ฟังก์ชันคำนวณสีของปุ่มเก้าอี้ตามสถานะที่แท้จริงและค่า Local SELECTED
const getSeatClass = (seat) => {
  // ถ้าเรากดเลือกอยู่ตัวนั้น ให้เปลี่ยนเป็นสีฟ้าทันที (Local SELECTED)
  if (localSelected.value.includes(seat.seat_no)) {
    return 'seat-selected'
  }
  
  // สลับสีตามสถานะที่ดึงมาจากฐานข้อมูล/WebSocket
  switch (seat.status) {
    case 'LOCKED': return 'seat-locked' // สีส้ม
    case 'BOOKED': return 'seat-booked' // สีเทา
    default: return 'seat-available'    // สีแดง
  }
}

// 4. ฟังก์ชันกดปุ่ม Next เพื่อขอล็อกที่นั่ง (All-or-Nothing) ย้ายไปหน้า p_confirm
const handleNextStep = async () => {
  if (localSelected.value.length === 0) return

  if (isHoldingActiveLock.value) {
    router.push('/confirm')
    return
  }

  try {
    errorMessage.value = ''
    // ยิงคำขอสร้าง Distributed Lock ไปยังหลังบ้าน Go
    const response = await bookingService.reserveSeats(authStore.defaultShowtimeId, localSelected.value)
    
    // บันทึกตั๋วที่เราล็อกสำเร็จและเวลาหมดอายุลงสโตร์ Pinia เพื่อแชร์ไปใช้หน้าถัดไป
    authStore.saveHoldingSeats(localSelected.value, response.data.locked_until)
    // authStore.selectedSeats = localSelected.value
    // authStore.lockedUntil = response.data.locked_until

    // ล็อกสำเร็จ ย้ายตัวเข้าสู่หน้า p_confirm
    router.push('/confirm')
  } catch (error) {
    console.error('Lock fail error:', error)
    // หากโดนคนอื่นแย่งตัดหน้าจองเก้าอี้บางตัวไปก่อน ระบบจะเด้งฟ้องทันที (Atomic All-or-Nothing)
    if (error.response && error.response.status === 409) {
      alert('ขออภัยด้วยครับ! มีเก้าอี้บางตัวที่คุณเลือกเพิ่งถูกผู้อื่นกดล็อกตัดหน้าไปในเสี้ยววินาทีนี้ ระบบจะทำการอัปเดตผังใหม่')
      fetchInitialSeatMap() // ดึงผังที่นั่งใหม่ทันทีเพื่อล้างจอ
      localSelected.value = [] // เคลียร์ค่าเลือกเก่าทิ้ง
    } else {
      errorMessage.value = 'เกิดข้อผิดพลาดในการเชื่อมต่อเซิร์ฟเวอร์'
    }
  }
}

const startLocalTimer = () => {
  // หากมีนาฬิกาเรือนเก่ารันค้างอยู่ ให้ทุบทิ้งก่อนเพื่อไม่ให้เวลาวิ่งทับกัน
  if (localTimerInterval) clearInterval(localTimerInterval)
  
  if (!authStore.lockedUntil) {
    isHoldingActiveLock.value = false
    return
  }

  // แปลงเวลาหมดอายุจากหลังบ้าน Go ให้เป็นมิลลิวินาทีสำหรับเปรียบเทียบ
  const targetTime = new Date(authStore.lockedUntil).getTime()

  localTimerInterval = setInterval(() => {
    const now = new Date().getTime()
    const difference = targetTime - now

    // ⏳ กรณีครบกำหนดเวลา 5 นาทีแล้ว (Timeout)
    if (difference <= 0) {
      clearInterval(localTimerInterval)
      isHoldingActiveLock.value = false
      authStore.clearHoldingSeats() // ล้างค่าความจำในเครื่อง
      localSelected.value = []      // ล้างปุ่มสีฟ้าออก
      fetchInitialSeatMap()         // ดึงผังที่นั่งสีแดงว่างล่าสุดกลับขึ้นจออัตโนมัติ ทวงคืนความปกติ ✨
      return
    }

    // หากเวลายังเหลืออยู่ ให้เปิดสวิตช์โหมด "ตรึงหน้าจอห้ามเมาส์คลิก" ค้างไว้
    isHoldingActiveLock.value = true
    
    // คำนวณตัดแบ่งนาทีและวินาทีออกโชว์บนตัวป้าย
    const minutes = Math.floor((difference % (1000 * 60 * 60)) / (1000 * 60))
    const seconds = Math.floor((difference % (1000 * 60)) / 1000)
    
    // จัดรูปแบบให้เป็นเลข 2 หลักเสมอ เช่น 04:09
    const mStr = minutes < 10 ? '0' + minutes : minutes
    const sStr = seconds < 10 ? '0' + seconds : seconds

    timeLeftStr.value = `${mStr}:${sStr}`
  }, 1000)
}

// 5. 📡 ดักฟังช่องสัญญาณ WebSocket เพื่อเปลี่ยนสีปุ่มเก้าอี้เรียลไทม์เมื่อคนอื่นทำรายการ
onMounted(async () => {
  // 1. บังคับยิง API ไปดึงผังเก้าอี้หลักจาก MongoDB ขึ้นจอให้เสร็จก่อนเป็นอันดับแรกสุด
  await fetchInitialSeatMap() 
  connect() // เปิดท่อ WebSocket รับสัญญาณเรียลไทม์

  // 2. หลังจากผังที่นั่งหลักขึ้นจอครบถ้วนแล้ว ค่อยตรวจสอบประวัติตั๋วเก่าค้างเครื่อง
  if (authStore.selectedSeats && authStore.selectedSeats.length > 0 && authStore.lockedUntil) {
    const remain = new Date(authStore.lockedUntil).getTime() - new Date().getTime()
    
    if (remain > 0) {
      console.log('[SEATMAP] Re-applying active lock for seats:', authStore.selectedSeats)
      // ผูกค่าเก้าอี้เก่า และเปิดสวิตช์สั่งตรึงปุ่มห้ามเมาส์คลิกตามโฟลว์ใหม่ทันที
      localSelected.value = [...authStore.selectedSeats]
      isHoldingActiveLock.value = true
      startLocalTimer() // เปิดระบบนาฬิกานับถอยหลังภายในหน้านี้
    } else {
      // หากตั๋วใบนั้นหมดอายุ 5 นาทีไปแล้ว สั่งล้างสิทธิ์ทิ้ง
      authStore.clearHoldingSeats()
      localSelected.value = []
    }
  }

  // 3. ติดตามการแจ้งเตือนจาก WebSocket (โค้ดดักจับสีเก้าอี้ชุดเดิมทำงานต่อ)
  const wsCheckInterval = setInterval(() => {
    if (latestEvent.value) {
      const ev = latestEvent.value
      
      seats.value = seats.value.map(s => {
        if (ev.seats.includes(s.seat_no)) {
          if (ev.event === 'SEATS_LOCKED') {
            return { ...s, status: 'LOCKED' }
          }
          if (ev.event === 'SEATS_RELEASED') {
            if (authStore.selectedSeats.some(x => ev.seats.includes(x))) {
              clearInterval(localTimerInterval)
              isHoldingActiveLock.value = false
              authStore.clearHoldingSeats()
              localSelected.value = []
            }
            return { ...s, status: 'AVAILABLE' }
          }
          if (ev.event === 'BOOKING_SUCCESS') {
            return { ...s, status: 'BOOKED' }
          }
          if (ev.event === 'SYSTEM_RESET_BY_ADMIN') {
            if (localTimerInterval) clearInterval(localTimerInterval)
            isHoldingActiveLock.value = false
            authStore.clearHoldingSeats()
            localSelected.value = []
            return { ...s, status: 'AVAILABLE' }
          }
        }
        return s
      })
      latestEvent.value = null 
    }
  }, 100)

  onUnmounted(() => {
    clearInterval(wsCheckInterval)
    if (localTimerInterval) clearInterval(localTimerInterval)
    disconnect() 
  })
})

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}
</script>

<template>
  <div class="seat-container">
    <div class="seat-card">
      <!-- ส่วนแสดงโปรไฟล์และสิทธิ์แอดมิน -->
      <div class="header-bar">
        <span class="user-info">👤 บัญชี: {{ authStore.email }}</span>
        <div class="nav-buttons">
          <button v-if="authStore.isAdmin" @click="router.push('/admin')" class="btn-admin">⚙️ หน้า Admin</button>
          <button @click="handleLogout" class="btn-logout">ออกจากระบบ</button>
        </div>
      </div>

      <h1 class="title">🎬 เลือกที่นั่งภาพยนตร์</h1>
      <div class="screen-indicator">🖥️ จอภาพยนตร์ (SCREEN)</div>

      <div v-if="isLoading" class="loading-text">กำลังโหลดผังที่นั่งเรียลไทม์...</div>
      <div v-else-if="errorMessage" class="error-text">{{ errorMessage }}</div>

      <!-- ผังตารางเก้าอี้ 2 แถว แถวละ 5 ตัว (A1-B5) ตามเงื่อนไขของโจทย์ -->
      <div v-else class="theater-map">
        <!-- แถว A -->
        <div class="seat-row">
          <div class="row-label">A</div>
          <div 
            v-for="seat in rowA" 
            :key="seat.seat_no" 
            :class="['seat-btn', getSeatClass(seat)]"
            @click="toggleSeatSelection(seat.seat_no, seat.status)"
          >
            {{ seat.seat_no }}
          </div>
        </div>

        <!-- แถว B -->
        <div class="seat-row">
          <div class="row-label">B</div>
          <div 
            v-for="seat in rowB" 
            :key="seat.seat_no" 
            :class="['seat-btn', getSeatClass(seat)]"
            @click="toggleSeatSelection(seat.seat_no, seat.status)"
          >
            {{ seat.seat_no }}
          </div>
        </div>
      </div>

      <!-- สรุปผลด้านล่างของหน้าจอตามข้อแนะนำ UI -->
      <div class="summary-section">
        <div class="color-badges">
          <span class="badge badge-av">ว่าง (แดง)</span>
          <span class="badge badge-sel">กำลังเลือก (ฟ้า)</span>
          <span class="badge badge-lock">ติดล็อก (ส้ม)</span>
          <span class="badge badge-book">จองแล้ว (เทา)</span>
        </div>

        <div class="summary-box">
          <p class="summary-text">
            <span v-if="isHoldingActiveLock" class="text-warning-msg">⏳ ที่นั่งที่คุณกำลังดำเนินการล็อกค้างไว้ (เหลือเวลาล็อก {{ timeLeftStr }}):</span>
            <span v-else>🎫 เก้าอี้ที่คุณเลือก:</span>
            
            <span v-if="localSelected.length > 0" class="highlight-seats">{{ localSelected.join(', ') }}</span>
            <span v-else class="empty-seats">ยังไม่ได้เลือกที่นั่ง</span>
          </p>
          <button 
            :disabled="localSelected.length === 0" 
            @click="handleNextStep" 
            :class="['btn-next', { 'btn-resume': isHoldingActiveLock }]"
          >
            <span v-if="isHoldingActiveLock">🔄 กลับไปดำเนินรายการต่อ</span>
            <span v-else>Next (เข้าสู่หน้ายืนยัน) -></span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.seat-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background-color: #0f172a;
  color: #f8fafc;
  padding: 20px;
  font-family: system-ui, -apple-system, sans-serif;
}

.seat-card {
  background-color: #1e293b;
  border-radius: 16px;
  padding: 30px;
  max-width: 650px;
  width: 100%;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.4);
}

.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  color: #94a3b8;
  border-bottom: 1px solid #334155;
  padding-bottom: 12px;
  margin-bottom: 20px;
}

.nav-buttons {
  display: flex;
  gap: 8px;
}

.btn-admin { background-color: #0284c7; color: white; border: none; padding: 4px 10px; border-radius: 6px; cursor: pointer; }
.btn-logout { background-color: #ef4444; color: white; border: none; padding: 4px 10px; border-radius: 6px; cursor: pointer; }

.title { text-align: center; font-size: 22px; margin-bottom: 25px; }

.screen-indicator {
  background-color: #334155;
  text-align: center;
  padding: 8px;
  border-radius: 4px;
  font-size: 12px;
  letter-spacing: 2px;
  margin-bottom: 40px;
  border-bottom: 3px solid #64748b;
  color: #cbd5e1;
}

.theater-map {
  display: flex;
  flex-direction: column;
  gap: 20px;
  align-items: center;
  margin-bottom: 40px;
}

.seat-row {
  display: flex;
  gap: 15px;
  align-items: center;
}

.row-label {
  font-size: 18px;
  font-weight: 700;
  width: 25px;
  color: #94a3b8;
}

.seat-btn {
  width: 50px;
  height: 50px;
  display: flex;
  justify-content: center;
  align-items: center;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

/* 🎨 ระบบจับคู่คู่สีตามที่กำหนดเงื่อนไขไว้ */
.seat-available { background-color: #dc2626; color: white; border: 1px solid #b91c1c; } /* 🔴 AVAILABLE = แดง */
.seat-selected { background-color: #06b6d4; color: white; border: 1px solid #0891b2; box-shadow: 0 0 12px #06b6d4; } /* 🔵 SELECTED = ฟ้า */
.seat-locked { background-color: #f97316; color: white; border: 1px solid #ea580c; cursor: not-allowed; } /* 🟡 LOCKED = ส้ม */
.seat-booked { background-color: #4b5563; color: #9ca3af; border: 1px solid #374151; cursor: not-allowed; } /* ⚫ BOOKED = เทา */

.seat-btn:hover:not(.seat-locked):not(.seat-booked) { transform: scale(1.08); }

.disabled-click { cursor: not-allowed !important; opacity: 0.85; }

.color-badges { display: flex; justify-content: center; gap: 12px; font-size: 11px; margin-bottom: 25px; }
.badge { padding: 3px 8px; border-radius: 4px; }
.badge-av { background-color: #dc2626; }
.badge-sel { background-color: #06b6d4; }
.badge-lock { background-color: #f97316; }
.badge-book { background-color: #4b5563; }
.summary-box { background-color: #0f172a; padding: 15px 20px; border-radius: 10px; display: flex; justify-content: space-between; align-items: center; gap: 15px; }
.summary-text { font-size: 14px; color: #cbd5e1; margin: 0; line-height: 1.4; }
.highlight-seats { color: #06b6d4; font-weight: 700; font-size: 16px; margin-left: 5px; }
.empty-seats { color: #64748b; font-style: italic; }
.btn-next { background-color: #10b981; color: white; border: none; padding: 10px 20px; border-radius: 8px; font-weight: 600; cursor: pointer; transition: background 0.2s; white-space: nowrap; }
.btn-next:disabled { background-color: #334155; color: #64748b; cursor: not-allowed; }
.btn-next:hover:not(:disabled):not(.btn-resume) { background-color: #059669; }

.btn-resume { background-color: #f97316 !important; color: white !important; }
.btn-resume:hover { background-color: #ea580c !important; box-shadow: 0 0 12px rgba(249, 115, 22, 0.5); }
.text-warning-msg { color: #fdba74; font-weight: 700; }

.loading-text, .error-text { text-align: center; padding: 20px; color: #94a3b8; }
.error-text { color: #f87171; }
</style>