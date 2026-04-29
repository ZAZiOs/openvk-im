package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	env "ovk-im/src/config"
	"ovk-im/src/db"
	"ovk-im/src/redis"
	redisrepo "ovk-im/src/repo/redis"
	"ovk-im/src/repo/search"
	"ovk-im/src/transport/broadcaster"
	"ovk-im/src/transport/endpoints"
	"ovk-im/src/transport/endpoints/core"
	lp_trans "ovk-im/src/transport/longpoll"
)

func main() {
	log.Println("Starting OpenVK-IM server...")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	secret := env.Get("SECRET_KEY", "aaa")

	if len(secret) < 64 {
		log.Println("SECRET_KEY .env variable is less than 64 bytes")
		return
	}

	db.Connect()
	redis.Init()

	searchRepo := search.NewRepository(db.Instance, []byte(secret))
	lpBroadcaster := broadcaster.New()
	lpRepo := redisrepo.NewRepo(redis.Client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		pubsub := redis.Client.Subscribe(ctx, "lp_updates")
		defer pubsub.Close()

		for msg := range pubsub.Channel() {
			userID, _ := strconv.ParseInt(msg.Payload, 10, 64)
			lpBroadcaster.Notify(userID)
		}
	}()

	if !env.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	r.GET("/nim", func(c *gin.Context) {
		lp_trans.LongPollHandler(c.Writer, c.Request, lpBroadcaster, lpRepo)
	})

	endpointRouter := &endpoints.Router{
		BaseHandler: core.BaseHandler{
			DB:          db.Instance,
			LPRepo:      lpRepo,
			Broadcaster: lpBroadcaster,
			SearchRepo:  searchRepo,
		},
	}
	internal := r.Group("/method")
	{
		endpointRouter.Register(internal)
	}

	port := env.Get("APP_PORT", "8080")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("Server started on port %s", port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	sqlDB, _ := db.Instance.DB()
	sqlDB.Close()
	redis.Client.Close()
}
