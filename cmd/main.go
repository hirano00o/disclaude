package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hirano00o/disclaude/internal/bot"
	"github.com/hirano00o/disclaude/internal/config"
	"github.com/hirano00o/disclaude/internal/db"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// 環境変数の読み込み
	if err := godotenv.Load(); err != nil {
		logrus.WithError(err).Debug("No .env file found")
	}

	// 設定の読み込み
	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// ログレベルの設定
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// データベース接続の初期化
	dbConfig := db.DatabaseConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
	}
	database, err := db.NewConnection(dbConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}
	defer database.Close()

	// マイグレーションの実行
	if err := db.Migrate(database); err != nil {
		logrus.WithError(err).Fatal("Failed to run database migrations")
	}

	// Discord Botの初期化
	discordBot, err := bot.New(cfg, database)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create Discord bot")
	}

	// Botの開始
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := discordBot.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("Failed to start Discord bot")
	}

	logrus.Info("Discord Claude bot started successfully")

	// シグナルハンドリング
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// グレースフルシャットダウン
	logrus.Info("Shutting down Discord Claude bot...")
	cancel()
	discordBot.Stop()
	logrus.Info("Discord Claude bot stopped")
}
