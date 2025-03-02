package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	calc "github.com/furkan000/adhango/pkg/calc"
	data "github.com/furkan000/adhango/pkg/data"
	util "github.com/furkan000/adhango/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
)

type Config struct {
	Lan               float64 `toml:"lan"`
	Lon               float64 `toml:"lon"`
	CalculationMethod string  `toml:"calculation_method"`
	AdhanPrayer       bool    `toml:"adhan_prayer"`
	Volume            struct {
		Fajr        float64 `toml:"fajr"`
		Dhuhr       float64 `toml:"dhur"`
		Asr         float64 `toml:"asr"`
		Maghrib     float64 `toml:"maghrib"`
		Isha        float64 `toml:"isha"`
		AdhanPrayer float64 `toml:"adhan_prayer"`
		Sela        float64 `toml:"sela"`
	} `toml:"volume"`
}

var (
	scheduler   = gocron.NewScheduler(time.Local)
	config      Config
	madhab      = calc.SHAFI_HANBALI_MALIKI
	coordinates *util.Coordinates
)

func loadConfig() error {
	if _, err := toml.DecodeFile("config.toml", &config); err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}
	return nil
}

// audioFiles maps prayer names to their audio file paths.
var audioFiles = map[string]string{
	"fajr":    "audio/ezan1.mp3",
	"dhuhr":   "audio/ezan2.mp3",
	"asr":     "audio/ezan3.mp3",
	"maghrib": "audio/ezan4.mp3",
	"isha":    "audio/ezan5.mp3",
	"test":    "audio/test.mp3",
	"prayer":  "audio/prayer.mp3",
	"sela":    "audio/sela.mp3",
}

// getVolumeForPrayer returns the volume for a specific prayer type
func getVolumeForPrayer(prayerType string) float64 {
	switch prayerType {
	case "fajr":
		return config.Volume.Fajr
	case "dhuhr":
		return config.Volume.Dhuhr
	case "asr":
		return config.Volume.Asr
	case "maghrib":
		return config.Volume.Maghrib
	case "isha":
		return config.Volume.Isha
	case "prayer":
		return config.Volume.AdhanPrayer
	case "sela":
		return config.Volume.Sela
	default:
		return 100 // Default volume for test and unknown types
	}
}

// playAudio plays the specified MP3 file with volume adjustment.
func playAudio(filepath string, audioType string) error {
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("error opening audio file: %v", err)
	}
	defer f.Close()

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		return fmt.Errorf("error decoding MP3: %v", err)
	}
	defer streamer.Close()

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		return fmt.Errorf("error initializing speaker: %v", err)
	}

	// Calculate volume adjustment
	// Convert percentage to logarithmic scale where:
	// 0% = silence (very low volume, -4 is approximately -96dB)
	// 100% = normal volume (0 dB, no change)
	volume := getVolumeForPrayer(audioType)
	volumeAdjusted := &effects.Volume{
		Streamer: streamer,
		Base:     2,
		Volume:   -4 + (volume / 100.0 * 4), // Scale from -4 to 0
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(volumeAdjusted, beep.Callback(func() {
		done <- true
	})))

	<-done
	return nil
}

// testAudioOutput tests the audio system by playing the Fajr adhan.
func testAudioOutput() {
	fmt.Println("🔊 Testing audio output...")
	err := playAudio(audioFiles["fajr"], "fajr")
	if err != nil {
		log.Printf("❌ Audio test failed: %v\n", err)
	} else {
		fmt.Println("✅ Audio test successful! Test played.")
	}
}

// scheduleAdhan schedules the adhan playback for a specific prayer.
// The job is tagged "adhan" so that we can later remove only these jobs.
func scheduleAdhan(scheduler *gocron.Scheduler, prayerName string, prayerTime time.Time) {
	fmt.Printf("🕰️ Scheduling %s Adhan at %v\n", prayerName, prayerTime)
	// Use seconds precision in the formatted time.
	scheduler.Every(1).Day().LimitRunsTo(1).
		At(prayerTime.Format("15:04:05")).
		Tag("adhan"). // Tag the job for later removal.
		Do(func() {
			fmt.Printf("📢 Playing %s Adhan at %v\n", prayerName, prayerTime)
			err := playAudio(audioFiles[prayerName], prayerName)
			if err != nil {
				log.Printf("❌ Error playing %s Adhan: %v\n", prayerName, err)
				return
			}

			// Play prayer after adhan only for the five daily prayers if enabled
			if prayerName != "test" && config.AdhanPrayer {
				fmt.Printf("🤲 Playing prayer after %s Adhan\n", prayerName)
				err = playAudio(audioFiles["prayer"], "prayer")
				if err != nil {
					log.Printf("❌ Error playing prayer after %s Adhan: %v\n", prayerName, err)
				}
			}
		})
}

// updatePrayerTimes calculates and schedules prayer times for the current day.
func updatePrayerTimes(scheduler *gocron.Scheduler) {
	// Use global coordinates

	// Get current date.
	currentDate := time.Now()
	date := data.NewDateComponents(currentDate)

	method := getCalculationMethod(config.CalculationMethod)

	// Configure calculation parameters using builder.
	params := calc.NewCalculationParametersBuilder().
		SetMadhab(madhab).
		SetMethod(method).
		Build()

	// Calculate prayer times.
	prayerTimes, err := calc.NewPrayerTimes(coordinates, date, params)
	if err != nil {
		log.Printf("Error calculating prayer times: %v", err)
		return
	}

	// Set timezone to local.
	err = prayerTimes.SetTimeZone(currentDate.Location().String())
	if err != nil {
		log.Printf("Error setting timezone: %v", err)
		return
	}

	fmt.Println("📅 Today's Prayer Times:")
	fmt.Printf("🌅 Fajr: %v\n", prayerTimes.Fajr)
	fmt.Printf("☀️ Dhuhr: %v\n", prayerTimes.Dhuhr)
	fmt.Printf("🏙️ Asr: %v\n", prayerTimes.Asr)
	fmt.Printf("🌇 Maghrib: %v\n", prayerTimes.Maghrib)
	fmt.Printf("🌙 Isha: %v\n", prayerTimes.Isha)

	// Remove only the prayer time jobs (tagged "adhan").
	scheduler.RemoveByTag("adhan")

	// Schedule each prayer time.
	scheduleAdhan(scheduler, "fajr", prayerTimes.Fajr)
	scheduleAdhan(scheduler, "dhuhr", prayerTimes.Dhuhr)
	scheduleAdhan(scheduler, "asr", prayerTimes.Asr)
	scheduleAdhan(scheduler, "maghrib", prayerTimes.Maghrib)
	scheduleAdhan(scheduler, "isha", prayerTimes.Isha)
}

func testThreeSecondsFromNow(scheduler *gocron.Scheduler) {
	// Schedule a test job 3 seconds from now.
	t := time.Now().Add(3 * time.Second)
	fmt.Println("Scheduled time:", t)
	scheduleAdhan(scheduler, "fajr", t)
}

// updateSettingsHandler handles POST requests to update settings
func updateSettingsHandler(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.BindJSON(&updates); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Read current config
	var currentConfig map[string]interface{}
	configBytes, err := os.ReadFile("config.toml")
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read config file"})
		return
	}
	if _, err := toml.Decode(string(configBytes), &currentConfig); err != nil {
		c.JSON(500, gin.H{"error": "Failed to parse config file"})
		return
	}

	// Update config with new values
	for key, value := range updates {
		if key == "volume" {
			// Handle volume updates separately as it's a nested structure
			if volumeUpdates, ok := value.(map[string]interface{}); ok {
				if currentVolume, ok := currentConfig["volume"].(map[string]interface{}); ok {
					for k, v := range volumeUpdates {
						currentVolume[k] = v
					}
				}
			}
		} else {
			currentConfig[key] = value
		}
	}

	// Convert back to TOML and write to file
	f, err := os.Create("config.toml")
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to open config file for writing"})
		return
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(currentConfig); err != nil {
		c.JSON(500, gin.H{"error": "Failed to write config file"})
		return
	}

	// Call onUpdateSettings to apply changes
	onUpdateSettings()

	c.JSON(200, gin.H{"message": "Settings updated successfully"})
}

func main() {
	var err error
	// Load configuration
	if err = loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup Gin router
	router := gin.Default()
	router.POST("/settings", updateSettingsHandler)

	// Start HTTP server in a goroutine
	go func() {
		if err := router.Run(":8080"); err != nil {
			log.Printf("Failed to start HTTP server: %v", err)
		}
	}()

	// Initialize coordinates
	coordinates, err = util.NewCoordinates(config.Lan, config.Lon)
	if err != nil {
		log.Fatalf("Failed to initialize coordinates: %v", err)
	}

	// Schedule daily update at midnight.
	scheduler.Every(1).Day().At("00:00").Do(func() {
		updatePrayerTimes(scheduler)
	})

	// Run initial update.
	updatePrayerTimes(scheduler)

	// Optional Tests
	// testThreeSecondsFromNow(scheduler)
	// testAudioOutput()

	// Start the scheduler (blocking call).
	scheduler.StartBlocking()
}

// getCalculationMethod converts a string calculation method to its corresponding type
func getCalculationMethod(methodStr string) calc.CalculationMethod {
	switch methodStr {
	case "OTHER":
		return calc.OTHER
	case "MUSLIM_WORLD_LEAGUE":
		return calc.MUSLIM_WORLD_LEAGUE
	case "TURKEY":
		return calc.TURKEY
	case "EGYPTIAN":
		return calc.EGYPTIAN
	case "KARACHI":
		return calc.KARACHI
	case "UMM_AL_QURA":
		return calc.UMM_AL_QURA
	case "DUBAI":
		return calc.DUBAI
	case "MOON_SIGHTING_COMMITTEE":
		return calc.MOON_SIGHTING_COMMITTEE
	case "NORTH_AMERICA":
		return calc.NORTH_AMERICA
	case "KUWAIT":
		return calc.KUWAIT
	case "QATAR":
		return calc.QATAR
	case "SINGAPORE":
		return calc.SINGAPORE
	case "UOIF":
		return calc.UOIF
	default:
		log.Printf("Unknown calculation method %s, defaulting to TURKEY", methodStr)
		return calc.TURKEY
	}
}

func onUpdateSettings() {
	// Reload config file
	if err := loadConfig(); err != nil {
		log.Printf("Failed to reload config: %v", err)
		return
	}

	// Update coordinates if lat/lon changed
	var err error
	coordinates, err = util.NewCoordinates(config.Lan, config.Lon)
	if err != nil {
		log.Printf("Failed to update coordinates: %v", err)
		return
	}

	// Remove existing jobs and reschedule with new settings
	scheduler.RemoveByTag("adhan")
	updatePrayerTimes(scheduler)
}
