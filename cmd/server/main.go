package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"cx6/internal/config"
	"cx6/internal/handler"
	"cx6/internal/middleware"
	"cx6/internal/model"
	"cx6/internal/repository"
	"cx6/internal/service"
	"cx6/pkg/mysql"
	"cx6/pkg/redis"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := config.Load(configPath); err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if err := redis.Init(&config.AppConfig.Redis); err != nil {
		log.Fatalf("init redis failed: %v", err)
	}
	defer func() {
		if err := redis.Close(); err != nil {
			log.Printf("close redis failed: %v", err)
		}
	}()

	if err := mysql.Init(&config.AppConfig.MySQL); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	defer func() {
		if err := mysql.Close(); err != nil {
			log.Printf("close mysql failed: %v", err)
		}
	}()

	if err := mysql.AutoMigrate(&model.ScoreLog{}, &model.SeasonSettlement{}); err != nil {
		log.Printf("auto migrate mysql tables failed: %v", err)
	}

	gin.SetMode(config.AppConfig.Server.Mode)
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.RateLimitByIP(1000))

	redisRepo := repository.NewRankRedisRepo(config.AppConfig.Rank.LeaderboardKey)
	mysqlRepo := repository.NewRankMySQLRepo()
	rankService := service.NewRankService(redisRepo, mysqlRepo, &config.AppConfig.Rank)
	rankHandler := handler.NewRankHandler(rankService)

	v1 := r.Group("/api/v1")
	{
		rank := v1.Group("/rank")
		{
			rank.POST("/score/upload", rankHandler.UploadScore)
			rank.GET("/top", rankHandler.GetTop)
		}
		v1.GET("/health", rankHandler.HealthCheck)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"code": 40400, "msg": "route not found"})
	})

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	srv := &httpServer{}
	go func() {
		log.Printf("server starting on %s ...", addr)
		if err := srv.ListenAndServe(addr, r); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("shutdown signal received, shutting down server...")

	if err := srv.Shutdown(); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Printf("server exited")
}

type httpServer struct {
	engine *gin.Engine
}

func (s *httpServer) ListenAndServe(addr string, engine *gin.Engine) error {
	s.engine = engine
	return engine.Run(addr)
}

func (s *httpServer) Shutdown() error {
	return nil
}
