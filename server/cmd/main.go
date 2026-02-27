package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/maxence2997/delta-cast/server/internal/config"
	"github.com/maxence2997/delta-cast/server/internal/handler"
	"github.com/maxence2997/delta-cast/server/internal/middleware"
	"github.com/maxence2997/delta-cast/server/internal/provider"
	"github.com/maxence2997/delta-cast/server/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Providers
	agoraTokenProvider := provider.NewAgoraTokenProvider(cfg.AgoraAppID, cfg.AgoraAppCertificate)
	agoraMediaPushProvider := provider.NewAgoraMediaPushProvider(cfg.AgoraAppID, cfg.AgoraRegion, cfg.AgoraRESTKey, cfg.AgoraRESTSecret, cfg.AgoraTranscodingEnabled)
	agoraChannelNCSProvider := provider.NewAgoraChannelNCSProvider(cfg.AgoraChannelNCSSecret)
	agoraMediaPushNCSProvider := provider.NewAgoraMediaPushNCSProvider(cfg.AgoraMediaPushNCSSecret)
	gcpProvider := provider.NewGCPLiveStreamProvider(cfg.GCPProjectID, cfg.GCPRegion, cfg.GCPBucketName, cfg.GCPCDNDomain)
	youtubeProvider := provider.NewYouTubeProvider(cfg.YouTubeClientID, cfg.YouTubeClientSecret, cfg.YouTubeRefreshToken)

	// Service
	liveSvc := service.NewLiveService(agoraTokenProvider, agoraMediaPushProvider, gcpProvider, youtubeProvider)

	// Handlers
	liveHandler := handler.NewLiveHandler(liveSvc, cfg.AgoraAppID)
	webhookHandler := handler.NewWebhookHandler(liveSvc, agoraChannelNCSProvider, agoraMediaPushNCSProvider)

	// Router
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     cfg.CORSMethods,
		AllowHeaders:     cfg.CORSHeaders,
		ExposeHeaders:    cfg.CORSExposeHeaders,
		AllowCredentials: cfg.CORSAllowCredentials,
		MaxAge:           cfg.CORSMaxAge,
	}))
	r.Use(middleware.Logger())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/v1")
	{
		live := v1.Group("/live", middleware.JWTAuth(cfg.JWTSecret))
		{
			live.POST("/prepare", liveHandler.Prepare)
			live.POST("/start", liveHandler.Start)
			live.POST("/stop", liveHandler.Stop)
			live.GET("/status", liveHandler.Status)
		}

		v1.POST("/webhook/agora/channel", webhookHandler.HandleAgoraChannelEvent)
		v1.POST("/webhook/agora/media-push", webhookHandler.HandleAgoraMediaPushEvent)
	}

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
