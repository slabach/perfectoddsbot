package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"perfectOddsBot/models"
	"perfectOddsBot/scheduler"
	"perfectOddsBot/services"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/interactionService"
	"runtime/debug"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xo/dburl"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB
var err error
var discordToken string

func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
			debug.PrintStack()
		}
	}()

	for {
		runApp()
		log.Println("Restarting application after a crash...")
		time.Sleep(60 * time.Second) // Wait for a bit before restarting
	}
}

func runApp() {
	getEnv, ok := os.LookupEnv("ENV")
	if !ok {
		fmt.Println("ENV not found")
		return
	}

	// Connect to the database
	if getEnv == "production" {
		mysqlURL, ok := os.LookupEnv("MYSQL_URL")
		if !ok {
			fmt.Println("MYSQL_URL not found")
			return
		}

		u, err := dburl.Parse(mysqlURL + "?charset=utf8mb4&collation=utf8mb4_0900_ai_ci&parseTime=True&loc=Local")
		if err != nil {
			fmt.Println(err)
			return
		}

		db, err = gorm.Open(mysql.Open(u.DSN), &gorm.Config{})
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		connString := os.Getenv("MYSQL_URL")

		db, err = gorm.Open(mysql.Open(connString + "?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=True&loc=Local"))
		if err != nil {
			log.Fatalln(err)
			return
		}
	}

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

	err = db.AutoMigrate(
		&models.Bet{}, &models.BetEntry{}, &models.BetMessage{}, &models.ErrorLog{},
		&models.Guild{}, &models.User{}, &models.Migration{},
		&models.Parlay{}, &models.ParlayEntry{}, &models.UserInventory{},
	)
	if err != nil {
		log.Fatalf("Error migrating database: %v", err)
	}

	// Run historical stats migration
	err = services.ReRunHistoricalStatsMigration(db)
	if err != nil {
		log.Printf("Error running historical stats migration: %v", err)
		// Don't fatal here - allow app to continue even if migration fails
	}

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
	dg.AddHandler(messageReactionAdd)
	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		err := s.UpdateGameStatus(0, "Managing Bets!")
		if err != nil {
			return
		}
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening Discord session: %v", err)
	}

	// Run parlay resolution fix migration (after Discord session is ready so notifications can be sent)
	err = services.FixParlayResolutionMigration(dg, db)
	if err != nil {
		log.Printf("Error running parlay resolution migration: %v", err)
		// Don't fatal here - allow app to continue even if migration fails
	}
	defer func(dg *discordgo.Session) {
		err := dg.Close()
		if err != nil {

		}
	}(dg)

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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in messageCreate: %v", r)
		}
	}()

	if m == nil || m.Author == nil {
		return
	}

	if m.Author.Bot {
		return
	}

	guildID := m.GuildID
	userID := m.Author.ID

	// getting guild info to create the guild record if it doesn't exist already
	guild, err := guildService.GetGuildInfo(s, db, guildID, m.ChannelID)
	if err != nil {
		msg := fmt.Errorf("error getting guild info: %v", err)
		common.SendError(s, nil, msg, db)
		return
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.Error != nil {
		msg := fmt.Errorf("error fetching or creating user: %v", result.Error)
		common.SendError(s, nil, msg, db)
	}
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
	}

	// Update username from message author
	username := common.GetUsernameFromUser(m.Author)
	common.UpdateUserUsername(db, &user, username)

	user.Points += guild.PointsPerMessage
	db.Save(&user)
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// getting guild info to create the guild record if it doesn't exist already
	_, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
	}

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		services.HandleSlashCommand(s, i, db)
	case discordgo.InteractionMessageComponent:
		interactionService.HandleComponentInteraction(s, i, db)
	case discordgo.InteractionModalSubmit:
		interactionService.HandleModalSubmit(s, i, db)
	}
}

func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// 1. Fetch the message to identify the author and count reactions
	m, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		log.Printf("Error fetching message for reaction: %v", err)
		return
	}

	// 2. Ignore bot messages and self-reactions
	if m.Author.Bot || m.Author.ID == r.UserID {
		return
	}

	// 3. Get Guild Info
	guild, err := guildService.GetGuildInfo(s, db, r.GuildID, r.ChannelID)
	if err != nil {
		log.Printf("Error getting guild info: %v", err)
		return
	}

	// 4. Find or Create the User (Author of the message)
	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: m.Author.ID, GuildID: r.GuildID})
	if result.Error != nil {
		log.Printf("Error fetching user: %v", result.Error)
		return
	}

	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
	}

	// 5. Calculate Total Reactions
	totalReactions := 0
	for _, reaction := range m.Reactions {
		totalReactions += reaction.Count
	}

	// 6. Calculate Exponential Bonus
	// Award = P * 2^(totalReactions - 1)
	if totalReactions > 0 {
		multiplier := math.Pow(2, float64(totalReactions-1))
		bonusPoints := guild.PointsPerMessage * multiplier
		user.Points += bonusPoints
	}

	// Update username if needed
	if user.Username == nil || *user.Username != m.Author.Username {
		username := common.GetUsernameFromUser(m.Author)
		user.Username = &username
	}

	db.Save(&user)
}
