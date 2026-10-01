// Command video exports the demo canvas and its own soundtrack with DCK.
package main

import (
	"flag"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/video"
	"github.com/olivierh59500/go-jinx-rastermaster/internal/demo"
	"log"
	"time"
)

type recordingHost struct {
	*demo.Game
	tick int
}

func (h *recordingHost) Update() error {
	if h.tick == 3*demo.FPS {
		h.Game.Start()
	}

	h.tick++
	return h.Game.Update()
}
func (h *recordingHost) RecordingChapter() string {
	if h.tick <= 3*demo.FPS {
		return "Opening card"
	}
	return "Raster Master"
}

func main() {
	config := video.Config{Output: "recordings/raster-master.mp4", Title: "Raster Master Go", Width: demo.Width * 2, Height: demo.Height * 2, FPS: demo.FPS, TPS: demo.FPS, SampleRate: 48000, Duration: 3 * time.Minute, PosterAt: 26 * time.Second}
	config.Flags(flag.CommandLine)
	flag.Parse()
	if config.Duration <= 0 {
		log.Fatal("a looping intro requires a positive recording duration")
	}
	if err := video.Run(config, func() (ebiten.Game, error) {
		g, err := demo.NewGame(false)
		if err != nil {
			return nil, err
		}
		return &recordingHost{Game: g}, nil
	}); err != nil {
		log.Fatal(err)
	}
}
