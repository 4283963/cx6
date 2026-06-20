package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"cx6/internal/model"
	"cx6/pkg/mysql"
)

type RankMySQLRepo struct {
	db *gorm.DB
}

func NewRankMySQLRepo() *RankMySQLRepo {
	return &RankMySQLRepo{
		db: mysql.DB,
	}
}

func (r *RankMySQLRepo) CreateScoreLog(ctx context.Context, log *model.ScoreLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create score log failed: %w", err)
	}
	return nil
}

func (r *RankMySQLRepo) BatchCreateScoreLogs(ctx context.Context, logs []*model.ScoreLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(logs).Error; err != nil {
		return fmt.Errorf("batch create score logs failed: %w", err)
	}
	return nil
}

func (r *RankMySQLRepo) GetScoreLogsByPlayer(ctx context.Context, playerID string, seasonID int64, limit, offset int) ([]model.ScoreLog, int64, error) {
	var logs []model.ScoreLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ScoreLog{}).Where("player_id = ? AND season_id = ?", playerID, seasonID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count score logs failed: %w", err)
	}

	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("get score logs failed: %w", err)
	}

	return logs, total, nil
}

func (r *RankMySQLRepo) BatchCreateSettlements(ctx context.Context, settlements []*model.SeasonSettlement) error {
	if len(settlements) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(settlements).Error; err != nil {
		return fmt.Errorf("batch create settlements failed: %w", err)
	}
	return nil
}

func (r *RankMySQLRepo) GetSettlementBySeasonAndPlayer(ctx context.Context, seasonID int64, playerID string) (*model.SeasonSettlement, error) {
	var s model.SeasonSettlement
	err := r.db.WithContext(ctx).
		Where("season_id = ? AND player_id = ?", seasonID, playerID).
		First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get settlement failed: %w", err)
	}
	return &s, nil
}

func (r *RankMySQLRepo) GetSettlementTopN(ctx context.Context, seasonID int64, n int) ([]model.SeasonSettlement, error) {
	var list []model.SeasonSettlement
	err := r.db.WithContext(ctx).
		Where("season_id = ?", seasonID).
		Order("final_rank ASC").
		Limit(n).
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("get settlement topN failed: %w", err)
	}
	return list, nil
}
