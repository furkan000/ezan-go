package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	calc "github.com/furkan000/adhango/pkg/calc"
	data "github.com/furkan000/adhango/pkg/data"
	util "github.com/furkan000/adhango/pkg/util"
	"github.com/go-co-op/gocron"
)

var (
	scheduler      = gocron.NewScheduler(time.Local)
	coordinates, _ = util.NewCoordinates(52.52, 13.405)
	// Toggle for playing prayer after adhan
	playPrayerAfterAdhan = true
)

// audioFiles maps prayer names to their audio file paths.
var audioFiles = map[string]string{
	"fajr":    "audio/ezan1.mp3",
	"dhuhr":   "audio/ezan2.mp3",
	"asr":     "audio/ezan3.mp3",
	"maghrib": "audio/ezan4.mp3",
	"isha":    "audio/ezan5.mp3",
	"test":    "audio/test.mp3",
	"prayer":  "audio/prayer.mp3",
}

// playAudio plays the specified MP3 file.
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

// testAudioOutput tests the audio system by playing the Fajr adhan.
func testAudioOutput() {
	fmt.Println("🔊 Testing audio output...")
	err := playAudio(audioFiles["test"])
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
			err := playAudio(audioFiles[prayerName])
			if err != nil {
				log.Printf("❌ Error playing %s Adhan: %v\n", prayerName, err)
				return
			}

			// Play prayer after adhan only for the five daily prayers if enabled
			if prayerName != "test" && playPrayerAfterAdhan {
				fmt.Printf("🤲 Playing prayer after %s Adhan\n", prayerName)
				err = playAudio(audioFiles["prayer"])
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

	// Configure calculation parameters using builder.
	params := calc.NewCalculationParametersBuilder().
		SetMadhab(calc.SHAFI_HANBALI_MALIKI).
		SetMethod(calc.TURKEY).
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

func main() {
	// Schedule daily update at midnight.
	scheduler.Every(1).Day().At("00:00").Do(func() {
		updatePrayerTimes(scheduler)
	})

	// Run initial update.
	updatePrayerTimes(scheduler)

	// Optional Tests
	// testThreeSecondsFromNow(scheduler)
	testAudioOutput()

	// Start the scheduler (blocking call).
	scheduler.StartBlocking()
}

func onUpdateSettings() {
	scheduler.RemoveByTag("adhan")
	updatePrayerTimes(scheduler)
}
