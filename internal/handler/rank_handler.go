package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"cx6/internal/model"
	"cx6/internal/service"
)

type RankHandler struct {
	rankService *service.RankService
}

func NewRankHandler(rankService *service.RankService) *RankHandler {
	return &RankHandler{
		rankService: rankService,
	}
}

type apiResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, apiResponse{
		Code: 0,
		Msg:  "ok",
		Data: data,
	})
}

func fail(c *gin.Context, httpCode, code int, msg string) {
	c.JSON(httpCode, apiResponse{
		Code: code,
		Msg:  msg,
	})
}

func (h *RankHandler) UploadScore(c *gin.Context) {
	var req model.ScoreUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40000, "invalid request parameters: "+err.Error())
		return
	}

	resp, err := h.rankService.UploadScore(c.Request.Context(), &req)
	if err != nil {
		h.handleUploadError(c, err)
		return
	}

	success(c, resp)
}

func (h *RankHandler) handleUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrScoreOutOfRange):
		fail(c, http.StatusBadRequest, 40010, err.Error())
	case errors.Is(err, service.ErrGameAlreadyProcessed):
		fail(c, http.StatusConflict, 40901, err.Error())
	case errors.Is(err, service.ErrRateLimitExceeded):
		fail(c, http.StatusTooManyRequests, 42902, err.Error())
	case errors.Is(err, service.ErrInvalidSignature):
		fail(c, http.StatusUnauthorized, 40110, err.Error())
	case errors.Is(err, service.ErrTimestampExpired):
		fail(c, http.StatusUnauthorized, 40111, err.Error())
	default:
		fail(c, http.StatusInternalServerError, 50001, "upload score failed: "+err.Error())
	}
}

func (h *RankHandler) GetTop(c *gin.Context) {
	var req model.TopNRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, http.StatusBadRequest, 40000, "invalid request parameters: "+err.Error())
		return
	}

	resp, err := h.rankService.GetTopN(c.Request.Context(), &req)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50002, "get top list failed: "+err.Error())
		return
	}

	success(c, resp)
}

func (h *RankHandler) HealthCheck(c *gin.Context) {
	success(c, gin.H{
		"status":  "running",
		"service": "cx6-rank-service",
	})
}
