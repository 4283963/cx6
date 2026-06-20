package model

import "time"

type ScoreUploadRequest struct {
	PlayerID string `json:"player_id" binding:"required,min=1,max=64"`
	GameID   string `json:"game_id" binding:"required,min=1,max=64"`
	Score    int64  `json:"score" binding:"required"`
	Mode     int8   `json:"mode" binding:"required,gte=0,lte=10"`
	RoomID   string `json:"room_id" binding:"required,min=1,max=64"`
	Sign     string `json:"sign" binding:"required,min=1"`
	Timestamp int64 `json:"timestamp" binding:"required"`
}

type ScoreUploadResponse struct {
	PlayerID   string `json:"player_id"`
	NewScore   int64  `json:"new_score"`
	NewRank    int64  `json:"new_rank"`
	DeltaScore int64  `json:"delta_score"`
}

type TopNRequest struct {
	PlayerID string `form:"player_id" binding:"omitempty,max=64"`
}

type TopNResponse struct {
	TotalPlayers int64       `json:"total_players"`
	MyRank       *MyRankInfo `json:"my_rank,omitempty"`
	TopList      []RankItem  `json:"top_list"`
}

type RankItem struct {
	Rank     int64  `json:"rank"`
	PlayerID string `json:"player_id"`
	Score    int64  `json:"score"`
}

type MyRankInfo struct {
	Rank  int64 `json:"rank"`
	Score int64 `json:"score"`
}

type ScoreLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID   string    `gorm:"type:varchar(64);not null;index:idx_player_season" json:"player_id"`
	GameID     string    `gorm:"type:varchar(64);not null" json:"game_id"`
	RoomID     string    `gorm:"type:varchar(64);not null" json:"room_id"`
	Mode       int8      `gorm:"type:tinyint;not null" json:"mode"`
	DeltaScore int64     `gorm:"type:bigint;not null" json:"delta_score"`
	AfterScore int64     `gorm:"type:bigint;not null;index:idx_player_season" json:"after_score"`
	SeasonID   int64     `gorm:"type:bigint;not null;index:idx_player_season" json:"season_id"`
	Status     int8      `gorm:"type:tinyint;not null;default:1;index" json:"status"`
	Reason     string    `gorm:"type:varchar(255)" json:"reason"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

type SeasonSettlement struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SeasonID    int64     `gorm:"type:bigint;not null;uniqueIndex" json:"season_id"`
	PlayerID    string    `gorm:"type:varchar(64);not null;index:idx_season_rank" json:"player_id"`
	FinalScore  int64     `gorm:"type:bigint;not null;index:idx_season_rank" json:"final_score"`
	FinalRank   int64     `gorm:"type:bigint;not null;index:idx_season_rank" json:"final_rank"`
	AwardLevel  int8      `gorm:"type:tinyint;not null;default:0" json:"award_level"`
	AwardInfo   string    `gorm:"type:json" json:"award_info"`
	Status      int8      `gorm:"type:tinyint;not null;default:0" json:"status"`
	SettledAt   time.Time `gorm:"index" json:"settled_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ScoreLog) TableName() string {
	return "score_logs"
}

func (SeasonSettlement) TableName() string {
	return "season_settlements"
}
