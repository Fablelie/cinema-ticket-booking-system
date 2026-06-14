import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import SeatMap from '../views/SeatMap.vue'
import Confirm from '../views/Confirm.vue'
import AdminDashboard from '../views/AdminDashboard.vue'
import { useAuthStore } from '../store/auth' // นำเข้าสโตร์ส่วนกลางเพื่อใช้งานเช็กสิทธิ์

// 1. กำหนดรายชื่อเส้นทางนำทางและจับคู่กับหน้าเว็บ (Component Views) แต่ละหน้า
const routes = [
  {
    path: '/',
    redirect: '/login' // หากเข้าหน้าเว็บแรกสุด ให้ระบบเด้งไปหน้า Login อัตโนมัติ
  },
  {
    path: '/login',
    name: 'Login',
    component: Login
  },
  {
    path: '/seats',
    name: 'SeatMap',
    component: SeatMap,
    meta: { requiresAuth: true } // หน้านี้ต้องเข้าสู่ระบบก่อนถึงจะเปิดใช้งานได้
  },
  {
    path: '/confirm',
    name: 'Confirm',
    component: Confirm,
    meta: { requiresAuth: true } // หน้านี้ต้องเข้าสู่ระบบก่อนเช่นกัน
  },
  {
    path: '/admin',
    name: 'AdminDashboard',
    component: AdminDashboard,
    meta: { requiresAuth: true, requiresAdmin: true } // ต้องเข้าสู่ระบบและมีบทบาทเป็น ADMIN เท่านั้น
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 2. นำระบบด่านตรวจสิทธิ์มาสกัดกั้นก่อนสลับเปลี่ยนหน้าจอ (Navigation Guard)
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore() // ดึงค่า Token และสิทธิ์แบบ Dynamic จากคลังความปลอดภัย
  const token = authStore.token

  // ตรวจสอบกรณีหน้าเว็บที่บังคับว่าต้องยืนยันตัวตนก่อน (requiresAuth)
  if (to.matched.some(record => record.meta.requiresAuth)) {
    if (!token) {
      // หากไม่มี Token ในระบบ ให้เตะผู้ใช้กลับไปหน้า Login ทันที
      next({ name: 'Login' })
    } else if (to.matched.some(record => record.meta.requiresAdmin)) {
      // ตรวจสอบสิทธิ์กรณีความปลอดภัยสูงของหน้าแอดมิน (requiresAdmin)
      // เรียกใช้ตัวคำนวณสิทธิ์กลางจาก Pinia ที่ดึงค่ามาจากไฟล์ .env โดยตรง
      if (!authStore.isAdmin) {
        alert('สิทธิ์ของคุณไม่สามารถเข้าใช้งานหน้าแอดมินได้!')
        next({ name: 'SeatMap' }) // หากไม่ผ่านด่านแอดมิน ให้ส่งกลับไปหน้าจองเก้าอี้ปกติของผู้ใช้ทั่วไป
      } else {
        next() // ผ่านด่านตรวจสอบสิทธิ์แอดมินฉลุย อนุญาตให้เข้าได้
      }
    } else {
      next() // เข้าถึงหน้าจอระดับ USER ทั่วไปได้ปกติ
    }
  } else {
    // ปล่อยผ่านสำหรับหน้าเว็บที่ไม่จำเป็นต้องเข้าสู่ระบบ (เช่น หน้า Login เอง)
    next()
  }
})

export default router
