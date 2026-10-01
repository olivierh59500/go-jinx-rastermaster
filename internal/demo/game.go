// Package demo composes Raster Master's native clocks through DCK.
package demo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sound"
	playback "github.com/olivierh59500/democonstructionkit/sound/ebiten"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/go-jinx-rastermaster/assets"
	"github.com/olivierh59500/go-jinx-rastermaster/internal/source"
)

const Width, Height, FPS = 320, 200, 50

type Game struct {
	images                                    []*ebiten.Image
	stage, plain, smallMask, largeMask, title *ebiten.Image
	slices                                    [60][4]*ebiten.Image
	clock                                     *source.Clock
	rasters                                   *source.Rasters
	blitter                                   *sprites.ImageSlots
	large, small                              *scrolling.Scrolling
	meter                                     [2]*composite.GradientBars
	palette, titlePalette                     *composite.IndexedPalette
	shader                                    *ebiten.Shader
	colors                                    [800]float32
	materials                                 [1600]float32
	uniforms                                  map[string]any
	visual                                    *sound.Stream
	player                                    *playback.Player
	pcm                                       [960 * 8]byte
	levels                                    [3]byte
	tick, track                               int
	mute, started, closed                     bool
}

func data(name string) ([]byte, error) { return assets.Files.ReadFile("original/" + name) }
func rgb(word uint16) color.NRGBA {
	return color.NRGBA{R: byte(word>>8&7) * 34, G: byte(word>>4&7) * 34, B: byte(word&7) * 34, A: 255}
}
func putRGB(dst []float32, w uint16) {
	c := rgb(w)
	dst[0], dst[1], dst[2], dst[3] = float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, 1
}

func NewGame(mute bool) (_ *Game, err error) {
	g := &Game{mute: mute, track: 2}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	load := func(name string) (*ebiten.Image, error) {
		b, e := data(name)
		if e != nil {
			return nil, e
		}
		img, _, e := image.Decode(bytes.NewReader(b))
		if e != nil {
			return nil, e
		}
		texture := ebiten.NewImageFromImage(img)
		g.images = append(g.images, texture)
		return texture, nil
	}
	for _, name := range []string{"top-bank.png", "from-bank.png", "dma.png"} {
		if _, err = load(name); err != nil {
			return nil, err
		}
	}
	largeFont, err := load("font-main.png")
	if err != nil {
		return nil, err
	}
	smallFont, err := load("font-small.png")
	if err != nil {
		return nil, err
	}
	g.title, err = load("title.png")
	if err != nil {
		return nil, err
	}
	read := func(name string) []byte {
		if err != nil {
			return nil
		}
		var b []byte
		b, err = data(name)
		return b
	}
	logo, from, small, message := read("logo-wave.bin"), read("from-wave.bin"), read("scroll-wave.bin"), read("message-main.bin")
	if err != nil {
		return nil, err
	}
	g.clock, err = source.NewClock(logo, from, small, message)
	if err != nil {
		return nil, err
	}
	blue, black, positions := read("raster-blue.bin"), read("raster-black.bin"), read("raster-positions.bin")
	thin, wide, rolling, materials := read("raster-small.bin"), read("raster-wide.bin"), read("raster-rolling.bin"), read("raster-materials.bin")
	if err != nil {
		return nil, err
	}
	g.rasters, err = source.NewRasters(blue, black, positions, thin, wide, rolling, materials)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 400; i++ {
		putRGB(g.materials[i*4:], binary.BigEndian.Uint16(materials[i*2:]))
	}
	for glyph := range g.slices {
		for column := range g.slices[glyph] {
			x, y := glyph%10*32+column*8, glyph/10*32
			g.slices[glyph][column] = largeFont.SubImage(image.Rect(x, y, x+8, y+32)).(*ebiten.Image)
		}
	}
	g.large, err = scrolling.New(scrolling.Config{GlyphWindow: &scrolling.GlyphWindowConfig{Count: 40, Advance: 8, Glyph: func(slot int) scrolling.Glyph {
		c := g.clock.Columns[slot]
		if c.Blank {
			return scrolling.Glyph{}
		}
		return scrolling.Glyph{Image: g.slices[c.Glyph][c.Slice]}
	}}})
	if err != nil {
		return nil, err
	}
	text := read("message-small.bin")
	if err != nil {
		return nil, err
	}
	glyphs := make([]scrolling.Glyph, len(text))
	for i, r := range text {
		c := max(0, min(127, int(r)-32))
		x, y := c%16*8, c/16*7
		glyphs[i] = scrolling.Glyph{Image: smallFont.SubImage(image.Rect(x, y, x+8, y+7)).(*ebiten.Image), Advance: 8}
	}
	g.small, err = scrolling.New(scrolling.Config{Glyphs: glyphs})
	if err != nil {
		return nil, err
	}
	for i := range g.meter {
		bank := i
		g.meter[i], err = composite.NewGradientBars(composite.GradientBarsConfig{Columns: 3, X: float64(64 + bank*128), Baseline: 166, Step: 32, Width: 16, Top: color.NRGBA{R: 255, G: 255, B: 255, A: 255}, Bottom: color.NRGBA{R: 255, G: 255, B: 255, A: 255}, Level: func(column int) float64 {
			channel := column
			if bank == 1 {
				channel = 2 - column
			}
			return float64(g.levels[channel]&15)*4 + 1
		}})
		if err != nil {
			return nil, err
		}
	}
	g.blitter, err = sprites.NewImageSlots(sprites.ImageSlotsConfig{Images: g.images, MaxSlots: 60, Blend: ebiten.BlendCopy})
	if err != nil {
		return nil, err
	}
	g.stage = ebiten.NewImage(Width, Height)
	g.plain = ebiten.NewImage(Width, Height)
	g.smallMask = ebiten.NewImage(Width, Height)
	g.largeMask = ebiten.NewImage(Width, Height)
	colors := make([]color.NRGBA, 16)
	b := read("palette.bin")
	if err != nil {
		return nil, err
	}
	for i := range colors {
		colors[i] = rgb(binary.BigEndian.Uint16(b[i*2:]))
	}
	colors[1] = rgb(0x666)
	g.palette, err = composite.NewIndexedPalette(composite.IndexedPaletteConfig{Palette: colors, Channel: composite.BitplaneRed, Blend: ebiten.BlendCopy})
	if err != nil {
		return nil, err
	}
	b = read("title-palette.bin")
	if err != nil {
		return nil, err
	}
	for i := range colors {
		colors[i] = rgb(binary.BigEndian.Uint16(b[i*2:]))
	}
	g.titlePalette, err = composite.NewIndexedPalette(composite.IndexedPaletteConfig{Palette: colors, Channel: composite.BitplaneRed, Blend: ebiten.BlendCopy})
	if err != nil {
		return nil, err
	}
	g.shader, err = ebiten.NewShader([]byte(nativeShader))
	if err != nil {
		return nil, err
	}
	g.uniforms = map[string]any{"Colors": g.colors[:], "Materials": g.materials[:]}
	if err = g.SelectMusic(2); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Game) Start() {
	if g.started {
		return
	}
	g.started = true
	if g.player != nil {
		g.player.Play()
	}
}
func (g *Game) SelectMusic(track int) error {
	if track < 0 || track > 9 {
		return fmt.Errorf("invalid music selection")
	}
	b, e := data(fmt.Sprintf("music-%d.ym", track+1))
	if e != nil {
		return e
	}
	visual, e := sound.Open("music.ym", b, sound.Options{SampleRate: 48000, BlockFrames: 960, Loop: true})
	if e != nil {
		return e
	}
	var player *playback.Player
	if !g.mute {
		player, e = playback.Open(nil, "music.ym", b, sound.Options{SampleRate: 48000, BlockFrames: 960, Loop: true})
		if e != nil {
			visual.Close()
			return e
		}
	}
	if g.player != nil {
		g.player.Close()
	}
	if g.visual != nil {
		g.visual.Close()
	}
	g.player, g.visual, g.track = player, visual, track
	if g.started && player != nil {
		player.Play()
	}
	return nil
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	g.tick++
	touches := inpututil.AppendJustPressedTouchIDs(nil)
	if !g.started {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || len(touches) > 0 || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.Start()
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return ebiten.Termination
	}
	keys := [10]ebiten.Key{ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5, ebiten.KeyF6, ebiten.KeyF7, ebiten.KeyF8, ebiten.KeyF9, ebiten.KeyF10}
	for i, key := range keys {
		if inpututil.IsKeyJustPressed(key) {
			if e := g.SelectMusic(i); e != nil {
				return e
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) || inpututil.IsKeyJustPressed(ebiten.KeyInsert) {
		g.rasters.BlueBackground = !g.rasters.BlueBackground
	}
	for _, id := range touches {
		x, _ := ebiten.TouchPosition(id)
		if x < Width/2 {
			g.rasters.BlueBackground = !g.rasters.BlueBackground
		} else {
			if e := g.SelectMusic((g.track + 1) % 10); e != nil {
				return e
			}
		}
	}
	g.clock.Step()
	g.rasters.Step()
	for i := 0; i < 200; i++ {
		putRGB(g.colors[i*4:], g.rasters.Pixels[i])
	}
	if _, e := io.ReadFull(g.visual, g.pcm[:]); e != nil {
		return e
	}
	if r, ok := g.visual.YMRegisters(); ok {
		for i := range g.levels {
			g.levels[i] = r[i+8]
		}
	}
	g.stage.Fill(color.NRGBA{A: 255})
	var slots [60]sprites.ImageSlot
	for row := 0; row < 30; row++ {
		x, y := g.clock.LogoRow(row)
		slots[row] = sprites.ImageSlot{Image: 0, Source: image.Rect(x, y, x+320, y+1), Y: float64(row)}
		y = g.clock.FromRow(row)
		slots[30+row] = sprites.ImageSlot{Image: 1, Source: image.Rect(0, y, 128, y+1), X: 96, Y: float64(30 + row)}
	}
	if e := g.blitter.SetSlots(slots[:]); e != nil {
		return e
	}
	g.blitter.Draw(g.stage)
	if e := g.blitter.SetSlots([]sprites.ImageSlot{{Image: 2, X: 32, Y: 62}}); e != nil {
		return e
	}
	g.blitter.Draw(g.stage)
	g.smallMask.Clear()
	g.largeMask.Clear()
	for lane := 0; lane < 13; lane++ {
		state := scrolling.IdentityState()
		state.X = float64(g.clock.SmallX(lane))
		state.Y = float64(93 + lane*10)
		state.First = max(0, int(-state.X/8)-2)
		state.End = min(264, state.First+44)
		g.small.DrawAt(g.smallMask, state)
	}
	state := scrolling.IdentityState()
	state.End = 40
	state.Y = 168
	g.large.DrawAt(g.largeMask, state)
	for _, meter := range g.meter {
		meter.Draw(g.largeMask)
	}
	return nil
}

func (g *Game) Draw(dst *ebiten.Image) {
	if !g.started {
		_ = g.titlePalette.Draw(dst, g.title)
		return
	}
	_ = g.palette.Draw(g.plain, g.stage)
	op := ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{g.stage, g.smallMask, g.largeMask, g.plain}, Uniforms: g.uniforms, Blend: ebiten.BlendCopy}
	dst.DrawRectShader(Width, Height, g.shader, &op)
}
func (*Game) Layout(int, int) (int, int) { return Width, Height }
func (g *Game) Tick() int                { return g.tick }
func (g *Game) Close() {
	if g == nil || g.closed {
		return
	}
	g.closed = true
	if g.player != nil {
		g.player.Close()
	}
	if g.visual != nil {
		g.visual.Close()
	}
	if g.blitter != nil {
		g.blitter.Close()
	}
	if g.large != nil {
		g.large.Close()
	}
	if g.small != nil {
		g.small.Close()
	}
	for _, m := range g.meter {
		if m != nil {
			m.Close()
		}
	}
	if g.palette != nil {
		g.palette.Close()
	}
	if g.titlePalette != nil {
		g.titlePalette.Close()
	}
	if g.shader != nil {
		g.shader.Deallocate()
	}
	for _, img := range g.images {
		img.Deallocate()
	}
	for _, img := range []*ebiten.Image{g.stage, g.plain, g.smallMask, g.largeMask} {
		if img != nil {
			img.Deallocate()
		}
	}
}

const nativeShader = `//kage:unit pixels
package main
var Colors [200]vec4
var Materials [400]vec4
func Fragment(position vec4,source vec2,color vec4)vec4{
 p:=source-imageSrc0Origin()
 index:=int(clamp(floor(imageSrc0At(source).r*15+0.5),0,15))
 if p.y>=93 {index=index-index%2+int(step(0.5,imageSrc1At(source).a))}
 if p.y>=101 {index=index-(index/2)%2*2+2*int(step(0.5,imageSrc2At(source).a))}
 row:=int(clamp(floor(p.y),0,199))
 if index==0{return Colors[row]*color}
 if index==2{return Materials[200+row]*color}
 if index==3{return Materials[row]*color}
 if p.y>=93&&index==1{return vec4(204.0/255,204.0/255,204.0/255,1)*color}
 return imageSrc3At(source)*color
}
`
