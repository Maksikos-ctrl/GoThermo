package main

import (
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func scanAllKeys(pattern string) ([]string, error) {
	var (
		keys   []string
		cursor uint64
	)
	for {
		batch, next, err := redisClient.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			return keys, nil
		}
	}
}

func compactMessageList(key string) (int, error) {
	for attempt := 0; attempt < 3; attempt++ {
		removed := 0
		err := redisClient.Watch(ctx, func(tx *redis.Tx) error {
			items, err := tx.LRange(ctx, key, 0, -1).Result()
			if err != nil {
				return err
			}

			keep := make([]interface{}, 0, len(items))
			for _, it := range items {
				if it != "DELETED" {
					keep = append(keep, it)
				}
			}
			removed = len(items) - len(keep)
			if removed == 0 {
				return nil
			}

			_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
				p.Del(ctx, key)
				if len(keep) > 0 {
					p.RPush(ctx, key, keep...)
				}
				return nil
			})
			return err
		}, key)

		if err == redis.TxFailedErr {
			continue
		}
		return removed, err
	}
	return 0, fmt.Errorf("%s kept changing, skipped", key)
}

func CompactDeletedMessages() {
	keys, err := scanAllKeys("channel:*:messages")
	if err != nil {
		log.Printf("⚠️ Cleanup skipped (could not list channels): %v", err)
		return
	}

	total, lists := 0, 0
	for _, key := range keys {
		removed, err := compactMessageList(key)
		if err != nil {
			log.Printf("⚠️ Cleanup of %s failed: %v", key, err)
			continue
		}
		if removed > 0 {
			total += removed
			lists++
		}
	}

	if total > 0 {
		log.Printf("🧹 Removed %d deleted-message placeholders from %d channels", total, lists)
	}
}
