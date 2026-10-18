// Package qr is a minimal QR code encoder — just enough to show an
// otpauth:// link for two-factor setup: byte mode, error correction level M,
// versions 1–10 (up to 213 bytes). Kept in-house rather than adding a
// dependency for one screen. Structure follows ISO/IEC 18004; the layout
// and masking steps mirror the well-known Nayuki qrcodegen reference.
package qr

import (
	"errors"
	"fmt"
	"strings"
)

var ErrTooLong = errors.New("qr: data too long")

// Per-version layout at error correction level M, versions 1–10.
var (
	// totalCodewords is the number of 8-bit codewords the symbol holds.
	totalCodewords = [11]int{0, 26, 44, 70, 100, 134, 172, 196, 242, 292, 346}
	// eccPerBlock and numBlocks split those codewords into RS blocks.
	eccPerBlock = [11]int{0, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26}
	numBlocks   = [11]int{0, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5}
	// alignment lists alignment pattern centre coordinates.
	alignment = [11][]int{
		nil, nil,
		{6, 18}, {6, 22}, {6, 26}, {6, 30}, {6, 34},
		{6, 22, 38}, {6, 24, 42}, {6, 26, 46}, {6, 28, 50},
	}
)

const maxVersion = 10

// Code is an encoded symbol: Modules[y][x] is true for a dark module.
type Code struct {
	Size    int
	Modules [][]bool
}

// Encode picks the smallest version that fits data and returns the symbol.
func Encode(data []byte) (*Code, error) {
	version := 0
	for v := 1; v <= maxVersion; v++ {
		if 4+countBits(v)+8*len(data) <= 8*dataCodewords(v) {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, ErrTooLong
	}

	codewords := addECCAndInterleave(version, encodeData(version, data))

	c := newCode(version)
	c.drawFunctionPatterns(version)
	c.drawCodewords(codewords)

	best, bestPenalty := 0, -1
	for mask := range 8 {
		c.applyMask(mask)
		c.drawFormatBits(mask)
		if p := c.penalty(); bestPenalty < 0 || p < bestPenalty {
			best, bestPenalty = mask, p
		}
		c.applyMask(mask) // XOR again to undo
	}
	c.applyMask(best)
	c.drawFormatBits(best)

	return &Code{Size: c.size, Modules: c.modules}, nil
}

// SVGPath returns path data drawing every dark module as a 1×1 square,
// offset by a 4-module quiet zone; the full viewBox is Size+8 square.
func (c *Code) SVGPath() string {
	var b strings.Builder
	for y, row := range c.Modules {
		for x, dark := range row {
			if dark {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+4, y+4)
			}
		}
	}
	return b.String()
}

func countBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

func dataCodewords(version int) int {
	return totalCodewords[version] - eccPerBlock[version]*numBlocks[version]
}

// encodeData builds the data codewords: mode, length, payload, terminator, padding.
func encodeData(version int, data []byte) []byte {
	var bits bitBuffer
	bits.append(0b0100, 4) // byte mode
	bits.append(uint32(len(data)), countBits(version))
	for _, b := range data {
		bits.append(uint32(b), 8)
	}

	capacity := 8 * dataCodewords(version)
	bits.append(0, min(4, capacity-len(bits)))
	bits.append(0, (8-len(bits)%8)%8)
	for pad := uint32(0xEC); len(bits) < capacity; pad ^= 0xEC ^ 0x11 {
		bits.append(pad, 8)
	}

	out := make([]byte, len(bits)/8)
	for i, bit := range bits {
		if bit {
			out[i>>3] |= 1 << (7 - i&7)
		}
	}
	return out
}

// addECCAndInterleave splits data into blocks, appends Reed–Solomon ECC to
// each, and interleaves them in the order the symbol stores them.
func addECCAndInterleave(version int, data []byte) []byte {
	blocks := numBlocks[version]
	eccLen := eccPerBlock[version]
	shortLen := len(data) / blocks
	numShort := blocks - len(data)%blocks
	divisor := rsDivisor(eccLen)

	dataBlocks := make([][]byte, blocks)
	eccBlocks := make([][]byte, blocks)
	for i, k := 0, 0; i < blocks; i++ {
		n := shortLen
		if i >= numShort {
			n++
		}
		dataBlocks[i] = data[k : k+n]
		eccBlocks[i] = rsRemainder(dataBlocks[i], divisor)
		k += n
	}

	out := make([]byte, 0, totalCodewords[version])
	for i := 0; i <= shortLen; i++ {
		for _, b := range dataBlocks {
			if i < len(b) {
				out = append(out, b[i])
			}
		}
	}
	for i := range eccLen {
		for _, b := range eccBlocks {
			out = append(out, b[i])
		}
	}
	return out
}

type bitBuffer []bool

func (b *bitBuffer) append(val uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		*b = append(*b, (val>>i)&1 != 0)
	}
}

// --- Reed–Solomon over GF(2^8), polynomial 0x11D ---

func gfMul(x, y byte) byte {
	var z int
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= int((y>>i)&1) * int(x)
	}
	return byte(z)
}

// rsDivisor returns the generator polynomial of the given degree, highest
// coefficient first, leading 1 omitted.
func rsDivisor(degree int) []byte {
	result := make([]byte, degree)
	result[degree-1] = 1
	root := byte(1)
	for range degree {
		for j := range result {
			result[j] = gfMul(result[j], root)
			if j+1 < len(result) {
				result[j] ^= result[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}
	return result
}

func rsRemainder(data, divisor []byte) []byte {
	result := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[len(result)-1] = 0
		for i, d := range divisor {
			result[i] ^= gfMul(d, factor)
		}
	}
	return result
}

// --- Module placement ---

type code struct {
	size       int
	modules    [][]bool
	isFunction [][]bool
}

func newCode(version int) *code {
	size := version*4 + 17
	c := &code{size: size, modules: make([][]bool, size), isFunction: make([][]bool, size)}
	for i := range size {
		c.modules[i] = make([]bool, size)
		c.isFunction[i] = make([]bool, size)
	}
	return c
}

func (c *code) setFunction(x, y int, dark bool) {
	c.modules[y][x] = dark
	c.isFunction[y][x] = true
}

func (c *code) drawFunctionPatterns(version int) {
	for i := range c.size {
		c.setFunction(6, i, i%2 == 0)
		c.setFunction(i, 6, i%2 == 0)
	}

	c.drawFinder(3, 3)
	c.drawFinder(c.size-4, 3)
	c.drawFinder(3, c.size-4)

	pos := alignment[version]
	last := len(pos) - 1
	for i, x := range pos {
		for j, y := range pos {
			// Skip the three that would overlap finder patterns.
			if (i == 0 && j == 0) || (i == 0 && j == last) || (i == last && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					c.setFunction(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}

	c.drawFormatBits(0) // placeholder; reserves the area, redrawn per mask
	c.drawVersion(version)
}

func (c *code) drawFinder(x, y int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			xx, yy := x+dx, y+dy
			if xx < 0 || xx >= c.size || yy < 0 || yy >= c.size {
				continue
			}
			dist := max(abs(dx), abs(dy))
			c.setFunction(xx, yy, dist != 2 && dist != 4)
		}
	}
}

// drawFormatBits writes the 15-bit format info (level M = 0b00) in both copies.
func (c *code) drawFormatBits(mask int) {
	data := mask // level M's two bits are 00
	rem := data
	for range 10 {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return (bits>>i)&1 != 0 }

	for i := 0; i <= 5; i++ {
		c.setFunction(8, i, bit(i))
	}
	c.setFunction(8, 7, bit(6))
	c.setFunction(8, 8, bit(7))
	c.setFunction(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		c.setFunction(14-i, 8, bit(i))
	}

	for i := 0; i < 8; i++ {
		c.setFunction(c.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		c.setFunction(8, c.size-15+i, bit(i))
	}
	c.setFunction(8, c.size-8, true) // the always-dark module
}

func (c *code) drawVersion(version int) {
	if version < 7 {
		return
	}
	rem := version
	for range 12 {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	bits := version<<12 | rem
	for i := range 18 {
		dark := (bits>>i)&1 != 0
		a, b := c.size-11+i%3, i/3
		c.setFunction(a, b, dark)
		c.setFunction(b, a, dark)
	}
}

// drawCodewords fills non-function modules in the standard zigzag order:
// two-module-wide columns, right to left, alternating up and down.
func (c *code) drawCodewords(data []byte) {
	i := 0
	for right := c.size - 1; right >= 1; right -= 2 {
		if right == 6 { // skip the vertical timing pattern
			right = 5
		}
		for vert := range c.size {
			for j := range 2 {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = c.size - 1 - vert
				}
				if !c.isFunction[y][x] && i < len(data)*8 {
					c.modules[y][x] = (data[i>>3]>>(7-i&7))&1 != 0
					i++
				}
			}
		}
	}
}

func (c *code) applyMask(mask int) {
	for y := range c.size {
		for x := range c.size {
			if c.isFunction[y][x] {
				continue
			}
			var invert bool
			switch mask {
			case 0:
				invert = (x+y)%2 == 0
			case 1:
				invert = y%2 == 0
			case 2:
				invert = x%3 == 0
			case 3:
				invert = (x+y)%3 == 0
			case 4:
				invert = (x/3+y/2)%2 == 0
			case 5:
				invert = x*y%2+x*y%3 == 0
			case 6:
				invert = (x*y%2+x*y%3)%2 == 0
			case 7:
				invert = ((x+y)%2+x*y%3)%2 == 0
			}
			if invert {
				c.modules[y][x] = !c.modules[y][x]
			}
		}
	}
}

// penalty scores the symbol per the spec's four rules; lower is easier to scan.
func (c *code) penalty() int {
	n := c.size
	get := func(x, y int, transpose bool) bool {
		if transpose {
			return c.modules[x][y]
		}
		return c.modules[y][x]
	}

	score := 0
	for _, t := range []bool{false, true} {
		for y := range n {
			// Rule 1: runs of 5+ same-colour modules in a row/column.
			run := 1
			for x := 1; x < n; x++ {
				if get(x, y, t) == get(x-1, y, t) {
					run++
					continue
				}
				if run >= 5 {
					score += run - 2
				}
				run = 1
			}
			if run >= 5 {
				score += run - 2
			}

			// Rule 3: finder-like 1:1:3:1:1 patterns with 4 light modules on a side.
			for x := 0; x+11 <= n; x++ {
				var s [11]bool
				for k := range 11 {
					s[k] = get(x+k, y, t)
				}
				core := s[4] && !s[5] && s[6] && s[7] && s[8] && !s[9] && s[10]
				if core && !s[0] && !s[1] && !s[2] && !s[3] {
					score += 40
				}
				core = s[0] && !s[1] && s[2] && s[3] && s[4] && !s[5] && s[6]
				if core && !s[7] && !s[8] && !s[9] && !s[10] {
					score += 40
				}
			}
		}
	}

	// Rule 2: 2×2 blocks of one colour.
	dark := 0
	for y := range n {
		for x := range n {
			if c.modules[y][x] {
				dark++
			}
			if x+1 < n && y+1 < n {
				v := c.modules[y][x]
				if v == c.modules[y][x+1] && v == c.modules[y+1][x] && v == c.modules[y+1][x+1] {
					score += 3
				}
			}
		}
	}

	// Rule 4: deviation of the dark ratio from 50%, in 5% steps.
	total := n * n
	k := (abs(dark*20-total*10) + total - 1) / total
	return score + max(k-1, 0)*10
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
