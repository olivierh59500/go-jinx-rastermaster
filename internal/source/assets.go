package source

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const nativeImageSize = 196616

// prepare builds the original shifted logo banks without executing 68000 code.
func prepare(b []byte) []byte {
	m := make([]byte, 0x62268)
	copy(m, b)
	for part := 0; part < 4; part++ {
		off := int(binary.BigEndian.Uint32(b[0x5d0a+part*4:]))
		for row := 0; row < 30; row++ {
			copy(m[0x300a8+row*64+part*16:], b[0xbc66+off+row*160:0xbc66+off+row*160+16])
		}
	}
	for part := 0; part < 8; part++ {
		off := int(binary.BigEndian.Uint32(b[0x5cea+part*4:]))
		for row := 0; row < 30; row++ {
			copy(m[0x308c8+row*5120+part*16:], b[0xbc66+off+row*160:0xbc66+off+row*160+16])
		}
	}
	for shift := 1; shift < 16; shift++ {
		for row := 0; row < 30; row++ {
			at := 0x308c8 + row*5120 + shift*320
			copy(m[at:at+160], m[at-320:at-160])
			for plane := 0; plane < 4; plane++ {
				carry := uint16(0)
				for word := 0; word < 20; word++ {
					p := at + word*8 + plane*2
					v := binary.BigEndian.Uint16(m[p:])
					binary.BigEndian.PutUint16(m[p:], v>>1|carry<<15)
					carry = v & 1
				}
			}
		}
	}
	return m
}

// ExportAssets writes artwork, fonts and authored tables, never executable code.
func ExportAssets(prg []byte, directory string) error {
	if len(prg) < 28+nativeImageSize || binary.BigEndian.Uint16(prg) != 0x601a {
		return fmt.Errorf("source: incomplete Raster Master executable")
	}
	b := prg[28 : 28+nativeImageSize]
	m := prepare(b)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	write := func(name string, data []byte) error { return os.WriteFile(filepath.Join(directory, name), data, 0644) }
	for _, p := range []struct {
		name       string
		start, end int
	}{
		{"palette.bin", 0xbc46, 0xbc66}, {"title-palette.bin", 0x3cd8, 0x3cf8},
		{"logo-wave.bin", 0x3d88, 0x5618}, {"from-wave.bin", 0x5d3a, 0x795a},
		{"scroll-wave.bin", 0x7d84, 0xbc44},
		{"message-main.bin", 0x795a, 0x7c79}, {"message-small.bin", 0x7c7a, 0x7d82},
		{"raster-blue.bin", 0x59c0, 0x59c0 + 402}, {"raster-black.bin", 0x5b46, 0x5b46 + 402},
		{"raster-positions.bin", 0x5858, 0x59c0}, {"raster-small.bin", 0x5708, 0x5734},
		{"raster-rolling.bin", 0x5734, 0x5794}, {"raster-wide.bin", 0x5794, 0x57c0},
		{"raster-materials.bin", 0x16186, 0x164a6},
		{"font-main.bin", 0x14246, 0x14246 + 7680},
		{"font-small.bin", 0x13986, 0x13986 + 128*7},
	} {
		if err := write(p.name, b[p.start:p.end]); err != nil {
			return err
		}
	}
	if err := write("top-bank.bin", m[0x308c8-320:0x308c8+153600+160]); err != nil {
		return err
	}
	if err := write("from-bank.bin", m[0x300a8-64:0x300a8+1920+64]); err != nil {
		return err
	}
	decode := func(memory []byte, offset, width, height, stride int) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				var v byte
				for p := 0; p < 4; p++ {
					w := binary.BigEndian.Uint16(memory[offset+y*stride+x/16*8+p*2:])
					v |= byte(w>>uint(15-x%16)&1) << p
				}
				img.SetNRGBA(x, y, color.NRGBA{R: v * 17, A: 255})
			}
		}
		return img
	}
	images := map[string]*image.NRGBA{
		"top-bank.png":  decode(m, 0x308c8-320, 960, 481, 320),
		"from-bank.png": decode(m, 0x300a8-64, 128, 32, 64),
	}
	dma := make([]byte, 30*128)
	for part := 0; part < 8; part++ {
		off := int(binary.BigEndian.Uint32(b[0x5d1a+part*4:]))
		for row := 0; row < 30; row++ {
			copy(dma[row*128+part*16:], b[0xbc66+off+row*160:0xbc66+off+row*160+16])
		}
	}
	images["dma.png"] = decode(dma, 0, 256, 30, 128)
	if err := write("dma.bin", dma); err != nil {
		return err
	}
	title := make([]byte, 32000)
	copy(title, b[0xbc66:0xbc66+9600])
	for part := 0; part < 28; part++ {
		off := int(binary.BigEndian.Uint32(b[0x3cf8+part*4:]))
		for y := 0; y < 16; y++ {
			copy(title[off+y*160:off+y*160+2], b[0x3d68+y*2:0x3d68+y*2+2])
		}
	}
	images["title.png"] = decode(title, 0, 320, 200, 160)
	small := image.NewNRGBA(image.Rect(0, 0, 16*8, 8*7))
	for c := 0; c < 128; c++ {
		for y := 0; y < 7; y++ {
			for x := 0; x < 8; x++ {
				v := b[0x13986+c*7+y] >> uint(7-x) & 1
				small.SetNRGBA(c%16*8+x, c/16*7+y, color.NRGBA{R: 255, G: 255, B: 255, A: v * 255})
			}
		}
	}
	images["font-small.png"] = small
	large := image.NewNRGBA(image.Rect(0, 0, 10*32, 6*32))
	for c := 0; c < 60; c++ {
		off := int(binary.BigEndian.Uint32(b[0x5618+c*4:]))
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				v := b[0x14246+off+y*40+x/8] >> uint(7-x%8) & 1
				large.SetNRGBA(c%10*32+x, c/10*32+y, color.NRGBA{R: 255, G: 255, B: 255, A: v * 255})
			}
		}
	}
	images["font-main.png"] = large
	for name, img := range images {
		f, e := os.Create(filepath.Join(directory, name))
		if e != nil {
			return e
		}
		e = png.Encode(f, img)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
