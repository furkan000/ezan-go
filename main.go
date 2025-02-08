package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/go-co-op/gocron"
	calc "github.com/mnadev/adhango/pkg/calc"
	data "github.com/mnadev/adhango/pkg/data"
	util "github.com/mnadev/adhango/pkg/util"
)

// AdhanAudioFiles maps prayer names to their audio file paths
var adhanAudioFiles = map[string]string{
	"fajr":    "audio/ezan1.mp3",
	"dhuhr":   "audio/ezan2.mp3",
	"asr":     "audio/ezan3.mp3",
	"maghrib": "audio/ezan4.mp3",
	"isha":    "audio/ezan5.mp3",
}

// playAudio plays the specified MP3 file
func playAudio(filepath string) error {
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

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	<-done
	return nil
}

// testAudioOutput tests the audio system by playing the Fajr adhan
func testAudioOutput() {
	fmt.Println("🔊 Testing audio output...")
	err := playAudio(adhanAudioFiles["fajr"])
	if err != nil {
		log.Printf("❌ Audio test failed: %v\n", err)
	} else {
		fmt.Println("✅ Audio test successful! Fajr Adhan played.")
	}
}

// scheduleAdhan schedules the adhan playback for a specific prayer
func scheduleAdhan(scheduler *gocron.Scheduler, prayerName string, prayerTime time.Time) {
	scheduler.At(prayerTime.Format("15:04")).Do(func() {
		fmt.Printf("📢 Playing %s Adhan at %v\n", prayerName, prayerTime)
		err := playAudio(adhanAudioFiles[prayerName])
		if err != nil {
			log.Printf("❌ Error playing %s Adhan: %v\n", prayerName, err)
		}
	})
}

// updatePrayerTimes calculates and schedules prayer times for the current day
func updatePrayerTimes(scheduler *gocron.Scheduler) {
	// Define coordinates for Berlin
	coords, err := util.NewCoordinates(52.52, 13.405)
	if err != nil {
		log.Printf("Error creating coordinates: %v", err)
		return
	}

	// Get current date
	currentDate := time.Now()
	date := data.NewDateComponents(currentDate)

	// Configure calculation parameters using builder
	params := calc.NewCalculationParametersBuilder().
		SetMadhab(calc.HANAFI).
		SetMethod(calc.MUSLIM_WORLD_LEAGUE).
		Build()

	// Calculate prayer times
	prayerTimes, err := calc.NewPrayerTimes(coords, date, params)
	if err != nil {
		log.Printf("Error calculating prayer times: %v", err)
		return
	}

	// Set timezone to local
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

	// Schedule each prayer time
	scheduleAdhan(scheduler, "fajr", prayerTimes.Fajr)
	scheduleAdhan(scheduler, "dhuhr", prayerTimes.Dhuhr)
	scheduleAdhan(scheduler, "asr", prayerTimes.Asr)
	scheduleAdhan(scheduler, "maghrib", prayerTimes.Maghrib)
	scheduleAdhan(scheduler, "isha", prayerTimes.Isha)
}

func main() {
	scheduler := gocron.NewScheduler(time.Local)

	// Schedule daily update at midnight
	scheduler.Every(1).Day().At("00:00").Do(func() {
		updatePrayerTimes(scheduler)
	})

	// Run initial test and setup
	testAudioOutput()
	updatePrayerTimes(scheduler)

	// Start the scheduler
	scheduler.StartBlocking()
}
