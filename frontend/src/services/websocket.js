import { ref } from 'vue'

export const useWebSocket = () => {
  let ws = null
  // ตัวแปร Reactive สำหรับส่งข้อมูล Event ชุดล่าสุดออกไปให้หน้าจอ Vue 3 อัปเดตสีเก้าอี้ตามจริง
  const latestEvent = ref(null)

  // ฟังก์ชันเปิดการเชื่อมต่อ
  const connect = () => {
    ws = new WebSocket('ws://localhost:8080/api/v1/seats/ws')

    ws.onopen = () => {
      console.log('[WEBSOCKET] Connected to backend real-time server.')
    }

    // เมื่อได้รับสัญญาณพ่นข้อมูลมาจาก Hub หลังบ้าน Go
    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        latestEvent.value = data // ส่งข้อมูลไปให้คอมโพเนนต์เปลี่ยนสถานะสีบนหน้าจอ
      } catch (err) {
        console.error('[WEBSOCKET] Error parsing message:', err)
      }
    }

    // 🔄 กลไก Reconnect อัตโนมัติหากระบบเน็ตเวิร์กขัดข้องหรือหลังบ้านปิดตัวลงชั่วคราว
    ws.onclose = () => {
      console.log('[WEBSOCKET] Disconnected. Trying to reconnect in 3 seconds...')
      setTimeout(() => {
        connect()
      }, 3000)
    }

    ws.onerror = (error) => {
      console.error('[WEBSOCKET] Error detected:', error)
      ws.close()
    }
  }

  const disconnect = () => {
    if (ws) {
      ws.close()
    }
  }

  return {
    connect,
    disconnect,
    latestEvent
  }
}
