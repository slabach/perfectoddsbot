package main

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	_ "github.com/microsoft/go-mssqldb"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"perfectOddsBot/models"
	"perfectOddsBot/scheduler"
	"perfectOddsBot/services"
)

var db *gorm.DB

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	mysqlURL, ok := os.LookupEnv("MYSQL_URL")
	if ok == false {
		fmt.Println("MYSQL_URL not found")
		return
	}

	db, err = gorm.Open(mysql.Open(mysqlURL + "?charset=utf8&parseTime=True&loc=Local"))
	if err != nil {
		log.Fatalln(err)
		return
	}

	err = db.AutoMigrate(
		&models.Bet{}, &models.BetEntry{}, &models.ErrorLog{}, &models.User{},
	)
	if err != nil {
		log.Fatalf("Error migrating database: %v", err)
	}
}

func main() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatalf("DISCORD_BOT_TOKEN not set in environment variables")
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	dg.AddHandler(interactionCreate)
	dg.AddHandler(messageCreate)
	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		err := s.UpdateGameStatus(0, "Managing Bets!")
		if err != nil {
			return
		}
	})

	dg.Identify.Intents = discordgo.IntentsGuildPresences | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening Discord session: %v", err)
	}
	defer func(dg *discordgo.Session) {
		err := dg.Close()
		if err != nil {

		}
	}(dg)

	// Close the database connection when the main function finishes
	defer func(db *gorm.DB) {
		sqlDB, err := db.DB()
		if err != nil {
			log.Fatalln(err)
		}
		defer func(sqlDB *sql.DB) {
			err := sqlDB.Close()
			if err != nil {
				log.Fatalln(err)
			}
		}(sqlDB)
	}(db)

	err = services.RegisterCommands(dg)
	if err != nil {
		log.Fatalf("Error registering commands: %v", err)
	}

	// cron scheduled processes
	scheduler.SetupCron(dg, db)

	log.Println("Bot is running. Press CTRL+C to exit.")
	select {}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	guildID := m.GuildID
	userID := m.Author.ID

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.Error != nil {
		log.Printf("Error fetching or creating user: %v", result.Error)
		return
	}
	if result.RowsAffected == 1 {
		user.Points = 1000
	}

	user.Points += 1
	db.Save(&user)
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		services.HandleSlashCommand(s, i, db)
	case discordgo.InteractionMessageComponent:
		services.HandleComponentInteraction(s, i, db)
	case discordgo.InteractionModalSubmit:
		services.HandleModalSubmit(s, i, db)
	}
}
