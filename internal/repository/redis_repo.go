package repository

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"cx6/internal/model"
	redispkg "cx6/pkg/redis"
)

type RankRedisRepo struct {
	leaderboardKey string
}

func NewRankRedisRepo(leaderboardKey string) *RankRedisRepo {
	return &RankRedisRepo{
		leaderboardKey: leaderboardKey,
	}
}

func (r *RankRedisRepo) UploadScore(ctx context.Context, playerID string, deltaScore int64) (newScore int64, err error) {
	result, err := redispkg.ZIncrBy(ctx, r.leaderboardKey, float64(deltaScore), playerID)
	if err != nil {
		return 0, fmt.Errorf("redis zincrby failed: %w", err)
	}
	return int64(result), nil
}

func (r *RankRedisRepo) GetTopN(ctx context.Context, n int64) ([]model.RankItem, error) {
	zs, err := redispkg.ZRevRangeWithScores(ctx, r.leaderboardKey, 0, n-1)
	if err != nil {
		return nil, fmt.Errorf("redis zrevrange failed: %w", err)
	}

	items := make([]model.RankItem, 0, len(zs))
	for i, z := range zs {
		items = append(items, model.RankItem{
			Rank:     int64(i + 1),
			PlayerID: z.Member.(string),
			Score:    int64(z.Score),
		})
	}
	return items, nil
}

func (r *RankRedisRepo) GetPlayerRank(ctx context.Context, playerID string) (rank int64, score int64, err error) {
	rank, err = redispkg.ZRevRank(ctx, r.leaderboardKey, playerID)
	if err != nil {
		if err == goredis.Nil {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("redis zrevrank failed: %w", err)
	}

	scoreFloat, err := redispkg.ZScore(ctx, r.leaderboardKey, playerID)
	if err != nil {
		if err == goredis.Nil {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("redis zscore failed: %w", err)
	}

	return rank + 1, int64(scoreFloat), nil
}

func (r *RankRedisRepo) GetTotalPlayers(ctx context.Context) (int64, error) {
	count, err := redispkg.ZCard(ctx, r.leaderboardKey)
	if err != nil {
		return 0, fmt.Errorf("redis zcard failed: %w", err)
	}
	return count, nil
}

func (r *RankRedisRepo) IncrementUploadCounter(ctx context.Context, playerID string, windowSeconds int) (int64, error) {
	key := fmt.Sprintf("rank:upload:counter:%s", playerID)
	count, err := redispkg.IncrBy(ctx, key, 1)
	if err != nil {
		return 0, fmt.Errorf("redis incr counter failed: %w", err)
	}

	if count == 1 {
		_, err = redispkg.Expire(ctx, key, time.Duration(windowSeconds)*time.Second)
		if err != nil {
			return count, fmt.Errorf("redis expire counter failed: %w", err)
		}
	}
	return count, nil
}

func (r *RankRedisRepo) GetUploadCounter(ctx context.Context, playerID string) (int64, error) {
	key := fmt.Sprintf("rank:upload:counter:%s", playerID)
	val, err := redispkg.Get(ctx, key)
	if err != nil {
		if err == goredis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("redis get counter failed: %w", err)
	}
	count := int64(0)
	fmt.Sscanf(val, "%d", &count)
	return count, nil
}

func (r *RankRedisRepo) CheckAndMarkGameProcessed(ctx context.Context, gameID, playerID string, ttlSeconds int) (processed bool, err error) {
	key := fmt.Sprintf("rank:game:processed:%s:%s", gameID, playerID)
	ok, err := redispkg.SetNX(ctx, key, "1", time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		return false, fmt.Errorf("redis setnx game failed: %w", err)
	}
	return !ok, nil
}

const (
	riskBlacklistKey     = "rank:risk:blacklist"
	riskMaxScoreTTL     = 86400
	riskClusterWindowMs = 5000
	riskClusterThreshold = 5
	riskBlacklistTTL = 86400 * 7
)

type RiskRecord struct {
	TimestampMs int64
	Score        int64
	Mode         int8
	RoomID       string
}

func (r *RankRedisRepo) IsBlacklisted(ctx context.Context, playerID string) (bool, error) {
	score, err := redispkg.ZScore(ctx, riskBlacklistKey, playerID)
	if err != nil {
		if err == goredis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("redis zscore blacklist failed: %w", err)
	}
	return score > 0, nil
}

func (r *RankRedisRepo) AddToBlacklist(ctx context.Context, playerID string, reason string) error {
	now := float64(time.Now().Unix())
	_, err := redispkg.ZAdd(ctx, riskBlacklistKey, &goredis.Z{
		Score:  now,
		Member: playerID,
	})
	if err != nil {
		return fmt.Errorf("redis zadd blacklist failed: %w", err)
	}
	detailKey := fmt.Sprintf("rank:risk:blacklist:detail:%s", playerID)
	return redispkg.SetEX(ctx, detailKey, reason, time.Duration(riskBlacklistTTL)*time.Second)
}

func (r *RankRedisRepo) RecordMaxScoreAttempt(ctx context.Context, playerID string, score int64, mode int8, roomID string, timestampMs int64) (int64, error) {
	key := fmt.Sprintf("rank:risk:maxscore:%s", playerID)
	member := fmt.Sprintf("%d:%d:%d:%s", timestampMs, score, mode, roomID)
	_, err := redispkg.ZAdd(ctx, key, &goredis.Z{
		Score:  float64(timestampMs),
		Member: member,
	})
	if err != nil {
		return 0, fmt.Errorf("redis zadd maxscore failed: %w", err)
	}

	windowStart := timestampMs - riskClusterWindowMs
	min := fmt.Sprintf("%d", windowStart)
	max := fmt.Sprintf("%d", timestampMs)

	count, err := redispkg.ZCount(ctx, key, min, max)
	if err != nil {
		return 0, fmt.Errorf("redis zcount maxscore failed: %w", err)
	}

	_, _ = redispkg.Expire(ctx, key, time.Duration(riskMaxScoreTTL)*time.Second)
	return count, nil
}
