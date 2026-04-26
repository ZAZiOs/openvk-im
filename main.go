package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	env "ovk-im/src/config"
	"ovk-im/src/db"
	"ovk-im/src/redis"
	"ovk-im/src/transport/redis_listener"
)

func main() {
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

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Fatalf("Redis Listener panicked: %v", r)
			}
		}()
		redis_listener.Start()
	}()

	if !env.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	port := env.Get("APP_PORT", "8080")

	log.Printf("OpenVK-IM started on port %s", port)
	r.Run(":" + port)

	// Server stop
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	sqlDB, _ := db.Instance.DB()
	sqlDB.Close()

}
