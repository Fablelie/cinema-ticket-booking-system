package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repository struct {
	db    *mongo.Database
	redis *redis.Client
}

func NewRepository(db *mongo.Database, rdb *redis.Client) *Repository {
	return &Repository{db: db, redis: rdb}
}

// AcquireMultipleLocks พยายามล็อกเก้าอี้ทุกตัวแบบ All-or-Nothing
func (r *Repository) AcquireMultipleLocks(ctx context.Context, showtimeID string, seats []string, userID string, ttl time.Duration) (bool, error) {
	pipe := r.redis.TxPipeline()
	var lockedKeys []string

	// 1. ลองยิงคำสั่ง SetNX ผ่าน Pipeline เพื่อเพิ่มความเร็ว
	for _, seatNo := range seats {
		lockKey := fmt.Sprintf("lock:showtime:%s:seat:%s", showtimeID, seatNo)
		pipe.SetNX(ctx, lockKey, userID, ttl)
		lockedKeys = append(lockedKeys, lockKey)
	}

	// Exec คำสั่งทั้งหมดในรอบเดียว
	cmds, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return false, err
	}

	// 2. ตรวจสอบว่ามีเก้าอี้ตัวไหนล็อกไม่สำเร็จ (ได้ค่า false) หรือไม่
	allSuccess := true
	for _, cmd := range cmds {
		success, _ := cmd.(*redis.BoolCmd).Result()
		if !success {
			allSuccess = false
			break
		}
	}

	// 3. 💥 หัวใจของ Senior: ถ้ามีตัวใดตัวหนึ่งล้มเหลว ให้Rollback สั่งลบคีย์ที่แอบสร้างไว้ทันที!
	if !allSuccess {
		delPipe := r.redis.Pipeline()
		for _, key := range lockedKeys {
			// ใช้ Lua Script ตรวจสอบก่อนลบ เพื่อไม่ให้ไปลบมั่วซั่วโดนคีย์คนอื่น
			var luaRelease = redis.NewScript(`
				if redis.call("get",KEYS[1]) == ARGV[1] then
					return redis.call("del",KEYS[1])
				else
					return 0
				end
			`)
			luaRelease.Run(ctx, delPipe, []string{key}, userID)
		}
		_, _ = delPipe.Exec(ctx)
		return false, nil // ปฏิเสธคำขอนี้
	}

	return true, nil
}

// ReleaseMultipleLocks ใช้เมื่อผู้ใช้กด Cancel เพื่อปลดล็อกที่นั่งอย่างปลอดภัย
func (r *Repository) ReleaseMultipleLocks(ctx context.Context, showtimeID string, seats []string, userID string) error {
	pipe := r.redis.Pipeline()

	var luaRelease = redis.NewScript(`
		if redis.call("get",KEYS[1]) == ARGV[1] then
			return redis.call("del",KEYS[1])
		else
			return 0
		end
	`)

	for _, seatNo := range seats {
		lockKey := fmt.Sprintf("lock:showtime:%s:seat:%s", showtimeID, seatNo)
		luaRelease.Run(ctx, pipe, []string{lockKey}, userID)
	}

	_, err := pipe.Exec(ctx)
	return err
}
