//go:build gpu

package demo

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/fidelity/ebiten/testutil"
)

func TestMain(m *testing.M) { testutil.RunGPU(m) }

func TestDCKPlanesMatchNativeDisplay(t *testing.T) {
	g, e := NewGame(true)
	if e != nil {
		t.Fatal(e)
	}
	defer g.Close()
	g.Start()
	surface := ebiten.NewImage(Width, Height)
	defer surface.Deallocate()
	// Present between updates to retain the same bounded GPU command lifetime.
	pixels := make([]byte, Width*Height*4)
	for range 650 {
		if e = g.Update(); e != nil {
			t.Fatal(e)
		}
		g.Draw(surface)
		surface.ReadPixels(pixels)
	}
	f, e := os.Open("../source/testdata/frame-650.bin.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	expected, e := io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	var planes [3][]byte
	for i, img := range []*ebiten.Image{g.stage, g.smallMask, g.largeMask} {
		planes[i] = make([]byte, Width*Height*4)
		img.ReadPixels(planes[i])
	}
	differences := 0
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			at := (y*Width + x) * 4
			index := int(planes[0][at]) / 17
			if y >= 93 {
				index = index - index%2
				if planes[1][at+3] > 127 {
					index++
				}
			}
			if y >= 101 {
				index = index - (index/2)%2*2
				if planes[2][at+3] > 127 {
					index += 2
				}
			}
			native := 0
			for p := 0; p < 4; p++ {
				word := binary.BigEndian.Uint16(expected[y*160+x/16*8+p*2:])
				native |= int(word>>uint(15-x%16)&1) << p
			}
			if y >= 100 && y <= 165 {
				native &= ^2
				index &= ^2
			}
			if native != index {
				differences++
			}
		}
	}
	if differences != 0 {
		t.Fatalf("%d DCK pixels differ from native planes", differences)
	}
}
