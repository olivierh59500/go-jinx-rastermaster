package source

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/olivierh59500/go-jinx-rastermaster/assets"
)

func asset(t *testing.T, name string) []byte {
	t.Helper()
	b, e := assets.Files.ReadFile("original/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	f, e := os.Open("testdata/" + name + ".bin.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	b, e := io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func originalModel(t *testing.T) (*Model, *Rasters) {
	t.Helper()
	read := func(name string) []byte { return asset(t, name) }
	c, e := NewClock(read("logo-wave.bin"), read("from-wave.bin"), read("scroll-wave.bin"), read("message-main.bin"))
	if e != nil {
		t.Fatal(e)
	}
	font, message := read("font-small.bin"), read("message-small.bin")
	small := make([]byte, len(message)*7)
	for i, r := range message {
		copy(small[i*7:], font[(int(r)-32)*7:(int(r)-32)*7+7])
	}
	m := &Model{Clock: c, Top: read("top-bank.bin"), From: read("from-bank.bin"), DMA: read("dma.bin"), LargeFont: read("font-main.bin"), SmallFont: small}
	rasters, e := NewRasters(read("raster-blue.bin"), read("raster-black.bin"), read("raster-positions.bin"), read("raster-small.bin"), read("raster-wide.bin"), read("raster-rolling.bin"), read("raster-materials.bin"))
	if e != nil {
		t.Fatal(e)
	}
	return m, rasters
}

// Native memory captures exercise both initial playback and every wrap policy.
// Only the sound meter's plane is excluded because the YM playlist is replaced.
func TestNativeDisplayAndRasterCheckpoints(t *testing.T) {
	b, e := os.ReadFile("testdata/checkpoints.json")
	if e != nil {
		t.Fatal(e)
	}
	var points []struct{ Tick int }
	if e = json.Unmarshal(b, &points); e != nil {
		t.Fatal(e)
	}
	m, r := originalModel(t)
	for _, p := range points {
		for m.Clock.Tick < p.Tick {
			m.Step()
			r.Step()
		}
		t.Run(fmt.Sprint(p.Tick), func(t *testing.T) {
			expected := fixture(t, fmt.Sprintf("frame-%d", p.Tick))
			differences := 0
			for i, v := range m.Screen[:32000] {
				row, x := i/160, i%160
				if row >= 100 && row <= 165 && x%8 >= 2 && x%8 < 4 {
					continue
				}
				if v != expected[i] {
					differences++
				}
			}
			if differences != 0 {
				t.Fatalf("%d native plane bytes differ", differences)
			}
			palette := fixture(t, fmt.Sprintf("rasters-%d", p.Tick))
			for i, v := range r.Pixels[:200] {
				if v != binary.BigEndian.Uint16(palette[i*2:]) {
					t.Fatalf("raster row %d differs", i)
				}
			}
		})
	}
}

func TestAllLookupCropsRemainInsideCachedArt(t *testing.T) {
	m, r := originalModel(t)
	r.BlueBackground = false
	for range 12000 {
		m.Clock.Step()
		r.Step()
		for row := 0; row < 30; row++ {
			x, y := m.Clock.LogoRow(row)
			if x < 0 || x+320 > 960 || y < 0 || y >= 481 {
				t.Fatal("logo crop exceeds its cached bank")
			}
			y = m.Clock.FromRow(row)
			if y < 0 || y >= 32 {
				t.Fatal("FROM crop exceeds its bank")
			}
		}
		for lane := 0; lane < 13; lane++ {
			x := m.Clock.SmallX(lane)
			if x > 15 || x < -2112 {
				t.Fatal("parallax text exceeds the compiled message")
			}
		}
	}
}
