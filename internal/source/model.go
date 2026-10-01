package source

import "encoding/binary"

// Model is a CPU reference for the native plane writes. Desktop and Android
// use DCK renderers; this model validates those renderers against 68000 memory.
type Model struct {
	Clock                                *Clock
	Top, From, DMA, LargeFont, SmallFont []byte
	Screen                               [240 * 160]byte
}

func (m *Model) Step() {
	m.Clock.Step()
	c := m.Clock
	for row := 0; row < 30; row++ {
		x, y := c.LogoRow(row)
		at := y*320 + x/16*8
		copy(m.Screen[row*160:row*160+160], m.Top[at:at+160])
		from := c.FromRow(row) * 64
		copy(m.Screen[(30+row)*160+48:(30+row)*160+48+64], m.From[from:from+64])
	}
	for row := 0; row < 30; row++ {
		copy(m.Screen[(62+row)*160+16:(62+row)*160+16+128], m.DMA[row*128:row*128+128])
	}
	for lane := 0; lane < 13; lane++ {
		x := c.SmallX(lane)
		for row := 0; row < 7; row++ {
			for pixel := 0; pixel < 320; pixel++ {
				source := pixel - x
				glyph, dx := source/8, source%8
				bit := byte(0)
				if source >= 0 && glyph < len(m.SmallFont)/7 {
					bit = m.SmallFont[glyph*7+row] >> uint(7-dx) & 1
				}
				p := (93+lane*10+row)*160 + pixel/16*8 + pixel%16/8
				mask := byte(1 << uint(7-pixel%8))
				m.Screen[p] = m.Screen[p]&^mask | bit*mask
			}
		}
	}
	for slot, column := range c.Columns {
		for row := 0; row < 32; row++ {
			v := byte(0)
			if !column.Blank {
				glyph := column.Glyph
				off := glyph/10*1280 + glyph%10*4 + column.Slice
				v = m.LargeFont[off+row*40]
			}
			pixel := slot * 8
			at := (168+row)*160 + pixel/16*8 + pixel%16/8 + 2
			m.Screen[at] = v
		}
	}
}

// PlanarDMA converts the embedded indexed sprite back to its native words.
func PlanarDMA(indices []byte) []byte {
	dst := make([]byte, 30*128)
	for y := 0; y < 30; y++ {
		for x := 0; x < 256; x++ {
			v := indices[y*256+x]
			for p := 0; p < 4; p++ {
				at := y*128 + x/16*8 + p*2
				w := binary.BigEndian.Uint16(dst[at:])
				w |= uint16(v>>uint(p)&1) << uint(15-x%16)
				binary.BigEndian.PutUint16(dst[at:], w)
			}
		}
	}
	return dst
}
