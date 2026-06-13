<script setup>
import { RouterView } from 'vue-router'
</script>

<template>
  <main class="app-main">
    <RouterView />
  </main>
</template>

<style>
/* 🎨 Global Styles: กำหนดค่าเริ่มต้นของดีไซน์ทั้งหน้าบ้านให้สวยงามสอดคล้องกัน */
body {
  margin: 0;
  padding: 0;
  background-color: #0f172a; /* ธีมมืดดาร์กโหมดสไตล์โรงภาพยนตร์ */
  color: #f8fafc;
  font-family: system-ui, -apple-system, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* ตั้งค่าให้หน้าจอกลางยืดหยุ่นเต็มหน้าจอคอมพิวเตอร์ */
.app-main {
  min-height: 100vh;
  width: 100%;
  box-sizing: border-box;
}

/* บล็อกเอฟเฟกต์แอนิเมชันสั้นๆ เวลาสลับหน้าจอเว็บ */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
