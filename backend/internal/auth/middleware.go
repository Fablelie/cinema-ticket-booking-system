package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/fablelie/cinema-ticket-booking-system/config"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

// GoogleAuthMiddleware ทำหน้าที่ตรวจสอบความถูกต้องของ Google ID Token ที่ส่งมาจากหน้าบ้าน
func GoogleAuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. ดึงข้อมูลจาก Header "Authorization: Bearer <Token>"
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			c.Abort() // บล็อกไม่ให้คำขอวิ่งไปทำงานในชั้น Use Case
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header format"})
			c.Abort()
			return
		}

		idToken := parts[1]

		// 2. ตรวจสอบความพร้อมของค่าคอนฟิกระบบหลังบ้าน
		if cfg.GoogleClientID == "" || cfg.GoogleClientID == "your_google_client_id" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server configuration error: google client id is not properly configured"})
			c.Abort()
			return
		}

		// 3. 🔐 ตรวจสอบความปลอดภัยจริงผ่าน Google Server เท่านั้น (ไม่รองรับการจำลองข้อมูล)
		payload, err := idtoken.Validate(context.Background(), idToken, cfg.GoogleClientID)
		if err != nil {
			// หากเป็นโทเค็นปลอม หมดอายุ หรือข้อมูลผู้ใช้งานไม่ถูกต้อง ระบบจะสั่งปฏิเสธคำขอทันที
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: invalid or expired google oauth token"})
			c.Abort()
			return
		}

		// 4. แกะค่าเอกลักษณ์เฉพาะบุคคล (Google User ID ดิบ) และ Email ที่แท้จริงจาก Google Account
		userID := payload.Subject // 'sub' เป็นรหัสตัวเลขเฉพาะตัวที่ไม่ซ้ำกันของบัญชี Google
		userEmail, _ := payload.Claims["email"].(string)

		// 5. ฝังข้อมูลผู้ใช้งานลงไปใน Context ของ Gin เพื่อให้เลเยอร์ถัดไปดึงไปจองเก้าอี้ได้
		c.Set("user_id", userID)
		c.Set("user_email", userEmail)

		c.Next() // อนุญาตให้คำขอวิ่งผ่านไปหาโฟลว์ธุรกิจหลัก (Service/Handler)
	}
}

func AdminOnlyMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. ดึงอีเมลผู้ใช้ที่ถูกแกะไว้จากด่านแรก (GoogleAuthMiddleware)
		emailValue, exists := c.Get("user_email")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing user context"})
			c.Abort()
			return
		}

		email := emailValue.(string)

		// 2. 🛡️ ดึงรายชื่อแอดมินจากค่าคอนฟิก (.env) แล้วนำมาหั่นเป็น Array
		isAdmin := false
		adminList := strings.Split(cfg.AdminEmails, ",")

		for _, adminEmail := range adminList {
			// TrimSpace ป้องกันกรณีพิมพ์เว้นวรรคใน .env
			if strings.TrimSpace(adminEmail) == email {
				isAdmin = true
				break
			}
		}

		// 3. บล็อกทันทีหากไม่อยู่ในรายชื่อแอดมินของระบบ
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: user does not have admin privileges"})
			c.Abort()
			return
		}

		c.Next() // ผ่านฉลุย เข้าไปดึงข้อมูลแอดมินได้
	}
}
