<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../store/auth'
import { adminService } from '../services/api'
import apiClient from '../services/api' // นำเข้าสำหรับใช้คำสั่งยิงตรงกรณีฟังก์ชันพิเศษ

const router = useRouter()
const authStore = useAuthStore()

// State สำหรับตารางข้อมูลและตัวจัดการแสดงผล
const logs = ref([])
const isLoading = ref(false)
const errorMessage = ref('')

// Reactive Variables สำหรับผูกข้อมูลการคัดกรองกับหน้าจอ (UI Filters)
const filterUser = ref('')
const filterEvent = ref('')
const filterDate = ref('')

// 1. ฟังก์ชันดึงข้อมูล Audit Logs พร้อมแนบค่าพารามิเตอร์คัดกรองไปยังหลังบ้าน Go
const fetchDashboardData = async () => {
  try {
    isLoading.value = true
    errorMessage.value = ''
    
    // จัดกลุ่มพารามิเตอร์คัดกรองส่งพ่วงไปด้วยตาม Query Params
    const params = {}
    if (filterUser.value.trim()) params.user_id = filterUser.value.trim()
    if (filterEvent.value) params.event = filterEvent.value
    if (filterDate.value) params.date = filterDate.value

    const response = await adminService.getDashboardData(params)
    logs.value = response.data
  } catch (error) {
    console.error('Failed to fetch dashboard logs:', error)
    if (error.response && error.response.status === 403) {
      errorMessage.value = '🛑 สิทธิ์ของคุณถูกปฏิเสธ: บัญชีนี้ไม่มีสิทธิ์เข้าใช้งานหน้าผู้ดูแลระบบ'
    } else {
      errorMessage.value = 'ไม่สามารถโหลดข้อมูลแดชบอร์ดได้ กรุณาลองใหม่อีกครั้ง'
    }
  } finally {
    isLoading.value = false
  }
}

// 2. ⚡ ฟังก์ชันพลังแอดมิน: สั่งล้างเก้าอี้และปลดล็อกระบบทั้งหมดในคลิกเดียว (Senior UX Helper)
const handleResetSystem = async () => {
  if (!confirm('⚠️ คุณแน่ใจใช่ไหมที่จะล้างผังที่นั่งทั้งหมดกลับเป็น AVAILABLE?\nข้อมูลล็อกและการจองเดิมในรอบนี้จะถูกทำลายทั้งหมดเพื่อเริ่มต้นใหม่')) return

  try {
    isLoading.value = true
    // ยิงคำขอพิเศษระดับแอดมินเพื่อล้างฐานข้อมูลผังเก้าอี้
    await apiClient.post('/admin/reset-seats', { showtime_id: authStore.defaultShowtimeId })
    alert('⚡ เคลียร์ผังที่นั่งและล้าง Lock ระบบเรียบร้อย! เก้าอี้ 10 ตัวกลับเป็นสถานะ AVAILABLE (สีแดง) แล้ว')
    
    // ล้างหน้ากรองและรีโหลดข้อมูลเพื่อแสดง Log ล่าสุด
    clearFilters()
  } catch (error) {
    console.error('Reset error:', error)
    alert('ไม่สามารถรีเซ็ตระบบได้ กรุณาตรวจสอบการตั้งค่าความปลอดภัยหรือระดับสิทธิ์ของคุณ')
  } finally {
    isLoading.value = false
  }
}

// 3. ฟังก์ชันล้างค่าตัวกรองทั้งหมดกลับเป็นค่าเริ่มต้น
const clearFilters = () => {
  filterUser.value = ''
  filterEvent.value = ''
  filterDate.value = ''
  fetchDashboardData() // รีโหลดข้อมูลใหม่แบบดึงทั้งหมด
}

// 4. ฟังก์ชันจัดฟอร์แมตวันที่และเวลาให้อ่านง่ายสไตล์สากล
const formatDateTime = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('th-TH', { timeZone: 'Asia/Bangkok' })
}

// 5. ฟังก์ชันแยกแยะสีของข้อความ Event Tag ในตารางแดชบอร์ด
const getEventBadgeClass = (event) => {
  switch (event) {
    case 'BOOKING_SUCCESS': return 'badge-success'       // สีเขียว (จองถาวร)
    case 'SEATS_LOCKED': return 'badge-warning'          // สีส้ม (ติดล็อก)
    case 'SEATS_RELEASED': return 'badge-danger'         // สีแดง (หลุดล็อก/ยกเลิก)
    case 'BOOKING_TIMEOUT': return 'badge-warning'       // timeout
    case 'SYSTEM_RESET_BY_ADMIN': return 'badge-system'  // สีม่วง (แอดมินล้างกระดาน)
    case 'SYSTEM_LOCK_FAIL': return 'badge-lock-fail'    // ป้ายแจ้งเตือนแย่งตั๋วแพ้
    case 'SYSTEM_ERROR': return 'badge-error'            // ป้ายระบบขัดข้อง
    default: return 'badge-info'
  }
}

onMounted(() => {
  fetchDashboardData() // เรียกดึงข้อมูลครั้งแรกเมื่อเปิดหน้าจอนี้ขึ้นมา
})
</script>

<template>
  <div class="admin-container">
    <div class="admin-card">
      <!-- ส่วนแถบแสดงหัวเรื่องและโปรไฟล์นำทาง -->
      <div class="admin-header">
        <div class="title-area">
          <h1 class="main-title">📊 Admin Audit Logs Dashboard</h1>
          <p class="sub-title">ระบบตรวจสอบและบันทึกประวัติกิจกรรมสำคัญที่ถูกประมวลผลอซิงโครนัสผ่าน RabbitMQ (Requirement 2.2)</p>
        </div>
        <button @click="router.push('/seats')" class="btn-back">🍿 กลับไปหน้าผังที่นั่ง</button>
      </div>

      <!-- 🔍 โซนกล่องคัดกรองข้อมูลและปุ่มควบคุมพิเศษ (Dynamic Filters Grid) -->
      <div class="filter-grid">
        <!-- ตัวกรองที่ 1: คัดกรองตามรหัสผู้ใช้งาน -->
        <div class="filter-item">
          <label class="filter-label">Filter by User (Google ID)</label>
          <input 
            v-model="filterUser" 
            type="text" 
            placeholder="กรอก Google User ID..." 
            class="filter-input"
            @keyup.enter="fetchDashboardData"
          />
        </div>

        <!-- ตัวกรองที่ 2: คัดกรองตามประเภทเหตุการณ์ของตั๋วหนัง -->
        <div class="filter-item">
          <label class="filter-label">Filter by Status / Event</label>
          <select v-model="filterEvent" class="filter-input" @change="fetchDashboardData">
            <option value="">-- แสดงเหตุการณ์ทั้งหมด --</option>
            <option value="BOOKING_SUCCESS">🟢 BOOKING_SUCCESS (จองสำเร็จถาวร)</option>
            <option value="SEATS_LOCKED">🟡 SEATS_LOCKED (ติดล็อกค้างไว้)</option>
            <option value="SEATS_RELEASED">🔴 SEATS_RELEASED (หลุดจอง/ยกเลิก)</option>
            <option value="BOOKING_TIMEOUT">⏳ BOOKING_TIMEOUT (หมดเวลา/หลุดล็อก)</option>
            <option value="SYSTEM_RESET_BY_ADMIN">🟣 SYSTEM_RESET (แอดมินรีเซ็ต)</option>
            <option value="SYSTEM_LOCK_FAIL">🛑 SYSTEM_LOCK_FAIL (lock ที่นั่งไม่สำเร็จ)</option>
            <option value="SYSTEM_ERROR">🔥 SYSTEM_ERROR (ระบบขัดข้อง)</option>
          </select>
        </div>

        <!-- ตัวกรองที่ 3: คัดกรองคัดแยกตามปฏิทินวันที่ -->
        <div class="filter-item">
          <label class="filter-label">Filter by Date</label>
          <input 
            v-model="filterDate" 
            type="date" 
            class="filter-input" 
            @change="fetchDashboardData"
          />
        </div>

        <!-- โซนปุ่มดำเนินการคัดกรอง และปุ่ม Reset สำหรับ Admin -->
        <div class="filter-actions">
          <!-- <button @click="fetchDashboardData" class="btn-search">🔍 ค้นหา (Filter)</button> -->
          <button @click="clearFilters" class="btn-clear">ล้างค่า</button>
          <button @click="handleResetSystem" class="btn-danger-admin">⚡ Reset ผังที่นั่งว่างทั้งหมด</button>
        </div>
      </div>

      <!-- โซนกล่องแจ้งสถานะโหลดข้อมูล -->
      <div v-if="isLoading" class="status-box">กำลังประมวลผลคัดกรองข้อมูลจาก MongoDB...</div>
      <div v-else-if="errorMessage" class="status-box error-text">{{ errorMessage }}</div>

      <!-- 📊 ตารางแสดงผลประวัติกิจกรรม (Audit Logs Table) -->
      <div v-else class="table-responsive">
        <table class="logs-table">
          <thead>
            <tr>
              <th>เวลาทำรายการ (Timestamp)</th>
              <th>เหตุการณ์ (Event)</th>
              <th>เบอร์ที่นั่ง (Seats)</th>
              <th>รหัสผู้ใช้งาน (User ID)</th>
              <th>รหัสรอบฉาย (Showtime ID)</th>
              <th>รายละเอียด / หมายเหตุ (Details)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(log, index) in logs" :key="index">
              <td class="time-col">{{ formatDateTime(log.timestamp) }}</td>
              <td>
                <span :class="['event-badge', getEventBadgeClass(log.event)]">
                  {{ log.event }}
                </span>
              </td>
              <td class="seat-col">{{ log.seats ? log.seats.join(', ') : '-' }}</td>
              <td class="id-col" :title="log.user_id">{{ log.user_id || 'ระบบอัตโนมัติ/แอดมิน' }}</td>
              <td class="id-col text-muted">{{ log.showtime_id }}</td>
              <td class="error-msg-col" :class="{ 'text-danger-msg': log.error_msg }">
                {{ log.error_msg || '-' }}
              </td>
            </tr>
            <tr v-if="logs.length === 0">
              <td colspan="5" class="empty-row">❌ ไม่พบประวัติ Log กิจกรรมในระบบตามเงื่อนไขตัวกรองที่คุณกำหนด</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-container {
  min-height: 100vh;
  background-color: #0f172a;
  color: #f8fafc;
  padding: 30px;
  font-family: system-ui, -apple-system, sans-serif;
}

.admin-card {
  background-color: #1e293b;
  border-radius: 16px;
  padding: 30px;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.4);
}

.admin-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid #334155;
  padding-bottom: 20px;
  margin-bottom: 30px;
}

.main-title { font-size: 24px; font-weight: 700; margin: 0 0 6px 0; color: #f8fafc; }
.sub-title { font-size: 13px; color: #94a3b8; margin: 0; line-height: 1.4; }

.btn-back {
  background-color: #4b5563;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.2s;
  white-space: nowrap;
}
.btn-back:hover { background-color: #374151; }

.filter-grid {
  background-color: #0f172a;
  padding: 20px;
  border-radius: 12px;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 15px;
  margin-bottom: 30px;
  border: 1px solid #334155;
  align-items: flex-end;
}

.filter-item { display: flex; flex-direction: column; gap: 8px; }
.filter-label { font-size: 12px; color: #94a3b8; font-weight: 600; }
.filter-input {
  background-color: #1e293b;
  border: 1px solid #334155;
  color: white;
  padding: 10px;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
  width: 100%;
  box-sizing: border-box;
}
.filter-input:focus { border-color: #38bdf8; }

.filter-actions { display: flex; gap: 8px; flex-wrap: wrap; width: 100%; }
.btn-search {
  background-color: #0284c7;
  color: white;
  border: none;
  padding: 10px 14px;
  border-radius: 6px;
  font-weight: 600;
  cursor: pointer;
  font-size: 13px;
  flex: 2;
  white-space: nowrap;
}
.btn-search:hover { background-color: #0369a1; }

.btn-clear {
  background-color: #334155;
  color: #cbd5e1;
  border: none;
  padding: 10px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  flex: 1;
}
.btn-clear:hover { background-color: #475569; color: white; }

.btn-danger-admin {
  background-color: #b91c1c;
  color: white;
  border: none;
  padding: 10px 14px;
  border-radius: 6px;
  font-weight: 700;
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
}
.btn-danger-admin:hover { background-color: #991b1b; box-shadow: 0 0 10px rgba(220, 38, 38, 0.5); }

.table-responsive { overflow-x: auto; border-radius: 12px; border: 1px solid #334155; }
.logs-table { width: 100%; border-collapse: collapse; text-align: left; font-size: 13px; min-width: 700px; }
.logs-table th { background-color: #334155; color: #cbd5e1; padding: 14px 16px; font-weight: 600; }
.logs-table td { padding: 14px 16px; border-bottom: 1px solid #334155; color: #e2e8f0; }
.logs-table tr:last-child td { border-bottom: none; }
.logs-table tr:hover td { background-color: #24334d; }

.time-col { color: #38bdf8; font-family: monospace; font-size: 12px; white-space: nowrap; }
.seat-col { font-weight: 700; color: #06b6d4; font-size: 15px; }
.id-col { font-family: monospace; max-width: 150px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.error-msg-col {
  max-width: 280px;
  white-space: normal;
  word-break: break-word;
  font-size: 12px;
  color: #94a3b8;
}

.text-danger-msg {
  color: #fda4af !important; 
  font-style: italic;
}

.event-badge { padding: 4px 8px; border-radius: 6px; font-size: 11px; font-weight: 700; display: inline-block; text-align: center; }
.badge-success { background-color: #10b981; color: white; }
.badge-warning { background-color: #f97316; color: white; }
.badge-danger { background-color: #ef4444; color: white; }
.badge-system { background-color: #8b5cf6; color: white; }
.badge-lock-fail { background-color: #f43f5e; color: white; border: 1px solid #e11d48; }
.badge-error { background-color: #7f1d1d; color: #fca5a5; border: 1px solid #991b1b; animation: pulse 2s infinite; }

.status-box { text-align: center; padding: 40px; color: #94a3b8; font-style: italic; }
.error-text { color: #f87171; font-weight: 600; }
.empty-row { text-align: center; padding: 30px !important; color: #64748b; font-style: italic; }
.text-muted { color: #64748b; }
</style>
