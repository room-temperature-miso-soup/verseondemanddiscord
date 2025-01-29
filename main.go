package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// Configuration constants
const (
	Prefix         = "!"
	BibleAPIURL    = "https://bible-api.com/data/web/random"
	RequestTimeout = 10 * time.Second
	EnvFileName    = ".env"
)

// AppConfig holds application-wide configuration
type AppConfig struct {
	DiscordToken string
	Debug        bool
}

// BibleVerse represents the structured data from the Bible API
type BibleVerse struct {
	Translation map[string]interface{} `json:"translation"`
	RandomVerse map[string]interface{} `json:"random_verse"`
}

// loadConfiguration handles loading and validating application configuration
func loadConfiguration() (*AppConfig, error) {
	// Load environment variables from .env file
	err := godotenv.Load(EnvFileName)
	if err != nil {
		return nil, fmt.Errorf("error loading %s file: %w", EnvFileName, err)
	}

	// Retrieve and validate required configuration values
	config := &AppConfig{
		DiscordToken: os.Getenv("DISCORD_BOT_TOKEN"),
		Debug:        os.Getenv("DEBUG") == "true",
	}

	// Validate critical configuration
	if config.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN is required in %s", EnvFileName)
	}

	return config, nil
}

// configureLogging sets up logging based on configuration
func configureLogging(debug bool) {
	if debug {
		// Verbose logging for debugging
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmicroseconds)
	} else {
		// Minimal production logging
		log.SetFlags(log.Ldate | log.Ltime)
	}
}

// readyHandler logs when the bot successfully connects to Discord and sends a hello message
func readyHandler(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot connected as %s#%s (ID: %s)", s.State.User.Username, s.State.User.Discriminator, s.State.User.ID)
	for _, guild := range s.State.Guilds {
		log.Printf("Connected to guild: %s (ID: %s)", guild.Name, guild.ID)
	}
}

// SafeSend sends a message to the specified channel with error handling
func SafeSend(s *discordgo.Session, channelID, content string) {
	_, err := s.ChannelMessageSend(channelID, content)
	if err != nil {
		log.Printf("Error sending message to channel %s: %v", channelID, err)
	}
}

// SafeSendEmbed sends an embedded message with error handling
func SafeSendEmbed(s *discordgo.Session, channelID string, embed *discordgo.MessageEmbed) {
	_, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Printf("Embed send error in channel %s: %v", channelID, err)
	}
}

// getBibleVerse fetches a random Bible verse with robust error handling
func getBibleVerse() (*BibleVerse, error) {
	client := &http.Client{
		Timeout: RequestTimeout,
	}

	resp, err := client.Get(BibleAPIURL)
	if err != nil {
		return nil, fmt.Errorf("bible verse API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bible verse API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024)) // 10KB limit
	if err != nil {
		return nil, fmt.Errorf("error reading API response: %w", err)
	}

	var verse BibleVerse
	if err := json.Unmarshal(body, &verse); err != nil {
		return nil, fmt.Errorf("failed to parse verse data: %w", err)
	}

	return &verse, nil
}

// createVerseEmbed generates a rich, informative Discord embed
func createVerseEmbed(verse *BibleVerse) *discordgo.MessageEmbed {
	var builder strings.Builder

	builder.WriteString("**Translation Details:**\n")
	for key, value := range verse.Translation {
		builder.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
	}

	builder.WriteString("\n**Random Verse:**\n")
	for key, value := range verse.RandomVerse {
		builder.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
	}

	return &discordgo.MessageEmbed{
		Title:       "Daily Bible Verse 📖",
		Description: builder.String(),
		Color:       0x3498db,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// messageCreate handles incoming Discord messages dynamically using message context
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Log message details in the terminal
	log.Printf("Message received in channel %s from %s: %s", m.ChannelID, m.Author.Username, m.Content)

	// Check if the message starts with the command prefix
	if !strings.HasPrefix(m.Content, Prefix) {
		return
	}

	// Extract command and arguments
	content := strings.TrimPrefix(m.Content, Prefix)
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	// Handle different commands
	switch command {
	case "hello":
		// Respond dynamically to the channel the message was received from
		SafeSend(s, m.ChannelID, "Hello! I'm your Bible verse bot. Type !verse for a random verse!")

	case "ping":
		SafeSend(s, m.ChannelID, "Pong! 🏓")

	case "verse":
		// Fetch a random Bible verse
		verse, err := getBibleVerse()
		if err != nil {
			log.Printf("Verse retrieval error: %v", err)
			SafeSend(s, m.ChannelID, "Sorry, I couldn't retrieve a verse right now.")
			return
		}

		// Create and send an embedded message with the Bible verse
		embed := createVerseEmbed(verse)
		SafeSendEmbed(s, m.ChannelID, embed)

	default:
		// Handle unknown commands
		SafeSend(s, m.ChannelID, "Unknown command. Try !hello, !ping, or !verse")
	}
}

func main() {
	// Load application configuration
	config, err := loadConfiguration()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Configure logging based on debug setting
	configureLogging(config.Debug)

	// Create Discord session
	dg, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	// Register event handlers
	dg.AddHandler(readyHandler)  // Logs when the bot connects
	dg.AddHandler(messageCreate) // Handles incoming messages

	// Open WebSocket connection to Discord
	err = dg.Open()
	if err != nil {
		log.Fatalf("Cannot open Discord connection: %v", err)
	}
	defer func() {
		err := dg.Close()
		if err != nil {
			log.Printf("Error closing Discord connection: %v", err)
		}
	}()

	// Log startup information
	log.Println("Bible Verse Bot is now running. Press CTRL-C to exit.")

	// Wait for termination signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Received termination signal. Shutting down...")
}
