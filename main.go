package main

import (
	"log"
	"time"

	"DebtBot/bot"
	"DebtBot/config"
	"DebtBot/db"
)

func main() {
	cfg := config.LoadConfig()
	database := db.NewDB(cfg)
	defer database.Close()

	err := database.InitSchema()
	if err != nil {
		log.Fatalf("Error initializing database schema: %v", err)
	}

	debtBot, err := bot.NewBot(cfg, database)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	// Запуск горутины для отправки уведомлений каждый день в 9 утра
	go func() {
		for {
			now := time.Now()
			nextNotificationTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
			if nextNotificationTime.Before(now) {
				nextNotificationTime = nextNotificationTime.Add(24 * time.Hour) // Если 9 утра уже прошло, то на завтра
			}
			waitDuration := nextNotificationTime.Sub(now)
			time.Sleep(waitDuration)

			log.Println("Sending daily notifications...")
			debtBot.SendNotifications()
		}
	}()

	log.Println("Bot started. Listening for updates...")
	if err := debtBot.Start(); err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}
}
