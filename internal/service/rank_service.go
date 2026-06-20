package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"cx6/internal/config"
	"cx6/internal/model"
	"cx6/internal/repository"
)

var (
	ErrScoreOutOfRange     = errors.New("score delta out of allowed range")
	ErrGameAlreadyProcessed = errors.New("game already processed")
	ErrRateLimitExceeded   = errors.New("upload rate limit exceeded")
	ErrInvalidSignature    = errors.New("invalid request signature")
	ErrTimestampExpired    = errors.New("request timestamp expired")
	ErrPlayerBlacklisted   = errors.New("player is blacklisted for cheating")
	ErrClusterCheatDetected = errors.New("cluster cheat detected: burst identical max-score uploads")
)

const (
	AppSecretKey        = "CX6_RANK_SECRET_2026"
	TimestampTolerance  = 300
	ProcessedGameTTL    = 86400
	DefaultSeasonID     = 1
)

type RankService struct {
	redisRepo *repository.RankRedisRepo
	mysqlRepo *repository.RankMySQLRepo
	cfg       *config.RankConfig
}

func NewRankService(redisRepo *repository.RankRedisRepo, mysqlRepo *repository.RankMySQLRepo, cfg *config.RankConfig) *RankService {
	return &RankService{
		redisRepo: redisRepo,
		mysqlRepo: mysqlRepo,
		cfg:       cfg,
	}
}

func (s *RankService) UploadScore(ctx context.Context, req *model.ScoreUploadRequest) (*model.ScoreUploadResponse, error) {
	if err := s.validateRequest(ctx, req); err != nil {
		return nil, err
	}

	blacklisted, err := s.redisRepo.IsBlacklisted(ctx, req.PlayerID)
	if err != nil {
		log.Printf("[WARN] check blacklist failed: player=%s, err=%v", req.PlayerID, err)
	} else if blacklisted {
		s.logSuspectBehavior(ctx, req, "player already blacklisted")
		return nil, ErrPlayerBlacklisted
	}

	if req.Score >= s.cfg.AntiCheat.MaxScorePerGame {
		if cheat, reason := s.detectClusterCheat(ctx, req); cheat {
			_ = s.redisRepo.AddToBlacklist(ctx, req.PlayerID, reason)
			s.logSuspectBehavior(ctx, req, reason)
			log.Printf("[RISK] player blacklisted due to cluster cheat: player=%s, reason=%s", req.PlayerID, reason)
			return nil, ErrClusterCheatDetected
		}
	}

	processed, err := s.redisRepo.CheckAndMarkGameProcessed(ctx, req.GameID, req.PlayerID, ProcessedGameTTL)
	if err != nil {
		log.Printf("[WARN] check game processed failed: player=%s, game=%s, err=%v", req.PlayerID, req.GameID, err)
	} else if processed {
		return nil, ErrGameAlreadyProcessed
	}

	count, err := s.redisRepo.IncrementUploadCounter(ctx, req.PlayerID, s.cfg.AntiCheat.WindowSeconds)
	if err != nil {
		log.Printf("[WARN] increment upload counter failed: player=%s, err=%v", req.PlayerID, err)
	} else if count > int64(s.cfg.AntiCheat.MaxUploadPerMinute) {
		s.logSuspectBehavior(ctx, req, fmt.Sprintf("rate limit exceeded: %d/%d", count, s.cfg.AntiCheat.MaxUploadPerMinute))
		return nil, ErrRateLimitExceeded
	}

	newScore, err := s.redisRepo.UploadScore(ctx, req.PlayerID, req.Score)
	if err != nil {
		return nil, fmt.Errorf("upload score to redis failed: %w", err)
	}

	newRank, _, err := s.redisRepo.GetPlayerRank(ctx, req.PlayerID)
	if err != nil {
		log.Printf("[WARN] get player rank after upload failed: player=%s, err=%v", req.PlayerID, err)
	}

	go s.asyncWriteScoreLog(req, newScore)

	return &model.ScoreUploadResponse{
		PlayerID:   req.PlayerID,
		NewScore:   newScore,
		NewRank:    newRank,
		DeltaScore: req.Score,
	}, nil
}

func (s *RankService) detectClusterCheat(ctx context.Context, req *model.ScoreUploadRequest) (bool, string) {
	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	if req.Timestamp > 0 {
		nowMs = req.Timestamp * 1000
	}

	count, err := s.redisRepo.RecordMaxScoreAttempt(ctx, req.PlayerID, req.Score, req.Mode, req.RoomID, nowMs)
	if err != nil {
		log.Printf("[WARN] record max score attempt failed: player=%s, err=%v", req.PlayerID, err)
		return false, ""
	}

	if count >= 5 {
		return true, fmt.Sprintf("cluster cheat: %d identical max-score(%d) uploads within 5000ms window", count, req.Score)
	}
	return false, ""
}

func (s *RankService) validateRequest(ctx context.Context, req *model.ScoreUploadRequest) error {
	if req.Score > s.cfg.AntiCheat.MaxScorePerGame || req.Score < s.cfg.AntiCheat.MinScorePerGame {
		s.logSuspectBehavior(ctx, req, fmt.Sprintf("score out of range: %d, max=%d, min=%d",
			req.Score, s.cfg.AntiCheat.MaxScorePerGame, s.cfg.AntiCheat.MinScorePerGame))
		return ErrScoreOutOfRange
	}

	now := time.Now().Unix()
	if abs(now-req.Timestamp) > TimestampTolerance {
		return ErrTimestampExpired
	}

	expectedSign := s.generateSign(req)
	if req.Sign != expectedSign {
		s.logSuspectBehavior(ctx, req, fmt.Sprintf("sign mismatch: expected=%s, actual=%s", expectedSign, req.Sign))
		return ErrInvalidSignature
	}

	return nil
}

func (s *RankService) generateSign(req *model.ScoreUploadRequest) string {
	raw := fmt.Sprintf("%s|%s|%d|%d|%s|%d|%s",
		req.PlayerID, req.GameID, req.Score, req.Mode, req.RoomID, req.Timestamp, AppSecretKey)
	h := md5.Sum([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (s *RankService) GetTopN(ctx context.Context, req *model.TopNRequest) (*model.TopNResponse, error) {
	topList, err := s.redisRepo.GetTopN(ctx, int64(s.cfg.TopN))
	if err != nil {
		return nil, fmt.Errorf("get topN failed: %w", err)
	}

	total, err := s.redisRepo.GetTotalPlayers(ctx)
	if err != nil {
		log.Printf("[WARN] get total players failed: %v", err)
		total = 0
	}

	resp := &model.TopNResponse{
		TotalPlayers: total,
		TopList:      topList,
	}

	if req.PlayerID != "" {
		rank, score, err := s.redisRepo.GetPlayerRank(ctx, req.PlayerID)
		if err != nil {
			log.Printf("[WARN] get my rank failed: player=%s, err=%v", req.PlayerID, err)
		} else if rank > 0 {
			resp.MyRank = &model.MyRankInfo{
				Rank:  rank,
				Score: score,
			}
		}
	}

	return resp, nil
}

func (s *RankService) asyncWriteScoreLog(req *model.ScoreUploadRequest, afterScore int64) {
	ctx := context.Background()
	scoreLog := &model.ScoreLog{
		PlayerID:   req.PlayerID,
		GameID:     req.GameID,
		RoomID:     req.RoomID,
		Mode:       req.Mode,
		DeltaScore: req.Score,
		AfterScore: afterScore,
		SeasonID:   DefaultSeasonID,
		Status:     1,
	}
	if err := s.mysqlRepo.CreateScoreLog(ctx, scoreLog); err != nil {
		log.Printf("[ERROR] async write score log failed: player=%s, game=%s, err=%v",
			req.PlayerID, req.GameID, err)
	}
}

func (s *RankService) logSuspectBehavior(ctx context.Context, req *model.ScoreUploadRequest, reason string) {
	scoreLog := &model.ScoreLog{
		PlayerID:   req.PlayerID,
		GameID:     req.GameID,
		RoomID:     req.RoomID,
		Mode:       req.Mode,
		DeltaScore: req.Score,
		AfterScore: 0,
		SeasonID:   DefaultSeasonID,
		Status:     0,
		Reason:     reason,
	}
	if err := s.mysqlRepo.CreateScoreLog(context.Background(), scoreLog); err != nil {
		log.Printf("[ERROR] write suspect log failed: player=%s, reason=%s, err=%v",
			req.PlayerID, reason, err)
	}
	log.Printf("[CHEAT] suspect behavior detected: player=%s, game=%s, reason=%s",
		req.PlayerID, req.GameID, reason)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
