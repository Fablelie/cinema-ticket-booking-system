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
		// Rollback: safely delete keys using Lua script (use Eval directly to avoid cache issues)
		luaReleaseScript := `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`
		for _, key := range lockedKeys {
			// Use Eval directly instead of Script.Run() in pipeline
			_, _ = r.redis.Eval(ctx, luaReleaseScript, []string{key}, userID).Result()
		}
		return false, nil // ปฏิเสธคำขอนี้
	}

	return true, nil
}

// ReleaseMultipleLocks ใช้เมื่อผู้ใช้กด Cancel เพื่อปลดล็อกที่นั่งอย่างปลอดภัย
func (r *Repository) ReleaseMultipleLocks(ctx context.Context, showtimeID string, seats []string, userID string) error {
	_, err := r.releaseMultipleLocks(ctx, showtimeID, seats, userID)
	return err
}

func (r *Repository) ReleaseMultipleLocksWithCount(ctx context.Context, showtimeID string, seats []string, userID string) (int64, error) {
	return r.releaseMultipleLocks(ctx, showtimeID, seats, userID)
}

func (r *Repository) releaseMultipleLocks(ctx context.Context, showtimeID string, seats []string, userID string) (int64, error) {
	// Lua script to safely delete only if key is owned by this user
	luaReleaseScript := `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`

	var released int64

	// Execute each lock release directly (not in pipeline to avoid NOSCRIPT issues)
	for _, seatNo := range seats {
		lockKey := fmt.Sprintf("lock:showtime:%s:seat:%s", showtimeID, seatNo)

		// Use Eval directly instead of Script.Run() to avoid EVALSHA cache issues
		result, err := r.redis.Eval(ctx, luaReleaseScript, []string{lockKey}, userID).Result()
		if err != nil && err != redis.Nil {
			// Log but don't fail the entire operation - this seat's lock might have already expired
			fmt.Printf("[REDIS] Error releasing lock for seat %s: %v\n", seatNo, err)
			continue
		}

		if n, ok := result.(int64); ok && n > 0 {
			released += n
		}
	}

	return released, nil
}
