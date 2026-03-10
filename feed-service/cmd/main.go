package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/feed-service/internal/config"
	"github.com/instagram-clone/feed-service/internal/controller"
	"github.com/instagram-clone/feed-service/internal/middleware"
	"github.com/instagram-clone/feed-service/internal/service"
)

func main() {
	cfg := config.Load()

	svc := service.NewFeedService(cfg)
	ctrl := controller.NewFeedController(svc)

	r := gin.Default()
	r.Use(middleware.Logger(), middleware.CORS())

	api := r.Group("/api/v1")
	api.Use(middleware.Auth(cfg))
	{
		api.GET("/feed", ctrl.GetFeed)
	}

	r.GET("/api/v1/feed/health", ctrl.Health)

	fmt.Printf("Feed service on :%s\n", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}
