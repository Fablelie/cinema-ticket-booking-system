<script setup>
import { useRouter } from 'vue-router'
import { GoogleSignInButton } from 'vue3-google-signin'
import { useAuthStore } from '../store/auth'

const router = useRouter()
const authStore = useAuthStore()

// ฟังก์ชันทำงานเมื่อผู้ใช้เข้าสู่ระบบด้วย Google สำเร็จ (ได้รับ Token กลับมาจากกูเกิล)
const handleLoginSuccess = (response) => {
  const idToken = response.credential // นี่คือ ID Token (JWT) ตัวที่จะยิงไปหลังบ้าน Go
  
  if (!idToken) return

  // 🔍 ทำการถอดรหัส JWT ส่วนกลางที่หน้าบ้านดึงมา เพื่อแอบดู 'email' เอาไปใช้ตรวจสอบสิทธิ์เบื้องต้น
  try {
    const base64Url = idToken.split('.')[1]
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split('')
        .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    )
    const decoded = JSON.parse(jsonPayload)
    const userEmail = decoded.email

    // 💾 บันทึก Token และ Email ลงสโตร์ Pinia และ LocalStorage ผ่านฟังก์ชันกลาง
    authStore.setLoginSession(idToken, userEmail)

    console.log('[AUTH] Login successful! User email:', userEmail)

    // 🚀 ย้ายหน้าจอตาม Workflow: 
    // หากเป็นแอดมิน ให้พาไปหน้า Dashboard แต่ถ้าเป็นผู้ใช้ทั่วไป พาไปหน้าเลือกเก้าอี้ (p_seat)
    if (authStore.isAdmin) {
      router.push('/admin')
    } else {
      router.push('/seats')
    }
  } catch (error) {
    console.error('[AUTH ERROR] Failed to parse Google token:', error)
    alert('เกิดข้อผิดพลาดในการตรวจสอบข้อมูล บัญชีผู้ใช้ไม่ถูกต้อง')
  }
}

// ฟังก์ชันทำงานกรณีเปิดระบบผิดพลาดหรือผู้ใช้กดปิดหน้าต่าง Pop-up หนี
const handleLoginError = () => {
  console.error('[AUTH ERROR] Google Sign-In failed or was closed.')
  alert('ไม่สามารถเข้าสู่ระบบได้ กรุณาตรวจสอบสิทธิ์และลองใหม่อีกครั้ง')
}
</script>

<template>
  <div class="login-container">
    <div class="login-card">
      <div class="icon-section">
        🎬
      </div>
      <h1 class="title">Cinema Ticket Booking</h1>
      <p class="subtitle">กรุณาเข้าสู่ระบบด้วยบัญชี Google เพื่อดำเนินการเลือกที่นั่ง</p>
      
      <!-- ปุ่มสำเร็จรูป Google Sign-In พ่นขึ้นหน้าจอตามมาตรฐาน OAuth 2.0 -->
      <div class="button-wrapper">
        <GoogleSignInButton
          shape="pill"
          theme="filled_blue"
          size="large"
          @success="handleLoginSuccess"
          @error="handleLoginError"
        />
      </div>

      <div class="footer-note">
        * ระบบจองตั๋วภาพยนตร์ออนไลน์อัจฉริยะ คอนคอร์เรนซีปลอดภัย 100%
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ดีไซน์โครงสร้างหน้าจอคลีนๆ ธีมดาร์กโหมดโรงภาพยนตร์ */
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background-color: #0f172a; /* สีเทาเข้มน้ำเงิน */
  font-family: 'Sukumvit Set', 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  padding: 20px;
}

.login-card {
  background-color: #1e293b; /* สีการ์ดเทาสว่างขึ้นมาหน่อย */
  padding: 40px 30px;
  border-radius: 16px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3), 0 8px 10px -6px rgba(0, 0, 0, 0.3);
  text-align: center;
  max-width: 400px;
  width: 100%;
}

.icon-section {
  font-size: 48px;
  margin-bottom: 16px;
}

.title {
  color: #f8fafc;
  font-size: 24px;
  font-weight: 700;
  margin-bottom: 8px;
}

.subtitle {
  color: #94a3b8;
  font-size: 14px;
  margin-bottom: 32px;
  line-height: 1.5;
}

.button-wrapper {
  display: flex;
  justify-content: center;
  margin-bottom: 24px;
}

.footer-note {
  color: #64748b;
  font-size: 11px;
  border-top: 1px solid #334155;
  padding-top: 16px;
}
</style>
