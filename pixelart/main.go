// Gera os GIFs 192x192 do Clawd em pixel art para a minitela.
// Uso: go run . [-out out]
// Gera working.gif (Imagem 1), done.gif (Imagem 2), needs.gif (Imagem 3) e uma folha de previa de cada.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

const (
	grid  = 24 // celulas por lado
	scale = 8  // pixels por celula -> 192x192 (tamanho dos slots do tema)
	size  = grid * scale
)

// Indices da paleta.
const (
	cBg = iota
	cOrange
	cShade
	cEye
	cKeyBase
	cKeyHi
	cDot
	cDotDim
	cAlert
)

var palette = color.Palette{
	cBg:      color.RGBA{0x14, 0x16, 0x1c, 0xff},
	cOrange:  color.RGBA{215, 119, 87, 0xff}, // cor do Clawd
	cShade:   color.RGBA{176, 92, 66, 0xff},
	cEye:     color.RGBA{0x1e, 0x14, 0x14, 0xff},
	cKeyBase: color.RGBA{0x55, 0x5b, 0x69, 0xff},
	cKeyHi:   color.RGBA{0xd0, 0xd3, 0xdc, 0xff},
	cDot:     color.RGBA{0xee, 0xee, 0xf0, 0xff},
	cDotDim:  color.RGBA{0x3a, 0x3e, 0x4a, 0xff},
	cAlert:   color.RGBA{255, 208, 96, 0xff},
}

type canvas [grid][grid]uint8

func (c *canvas) rect(x0, y0, x1, y1 int, col uint8) { // inclusivo
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x >= 0 && x < grid && y >= 0 && y < grid {
				c[y][x] = col
			}
		}
	}
}

type pose struct {
	dy         int  // deslocamento vertical do corpo (pulo)
	armL, armR int  // deslocamento dos bracos em linhas (negativo = para cima)
	eyes       int  // 0 normal, 1 piscando, 2 feliz (^ ^)
}

// clawd desenha o mascote.
func clawd(c *canvas, p pose) {
	dy := p.dy
	c.rect(4, 7+dy, 19, 15+dy, cOrange)
	c.rect(4, 14+dy, 19, 15+dy, cShade)
	c.rect(2, 10+p.armL+dy, 3, 12+p.armL+dy, cOrange)
	c.rect(20, 10+p.armR+dy, 21, 12+p.armR+dy, cOrange)
	for _, x := range []int{6, 9, 14, 17} {
		c.rect(x, 16+dy, x+1, 18+dy, cShade)
	}
	switch p.eyes {
	case 1:
		c.rect(8, 10+dy, 9, 10+dy, cEye)
		c.rect(14, 10+dy, 15, 10+dy, cEye)
	case 2:
		for _, x := range []int{8, 13} {
			c.rect(x, 10+dy, x, 10+dy, cEye)
			c.rect(x+1, 9+dy, x+1, 9+dy, cEye)
			c.rect(x+2, 10+dy, x+2, 10+dy, cEye)
		}
	default:
		c.rect(8, 9+dy, 9, 11+dy, cEye)
		c.rect(14, 9+dy, 15, 11+dy, cEye)
	}
}

func keyboard(c *canvas, hi int) {
	c.rect(3, 20, 20, 22, cKeyBase)
	for i := 0; i < 6; i++ {
		x := 4 + i*3
		col := uint8(cKeyBase)
		if i == hi {
			col = cKeyHi
		}
		c.rect(x, 21, x+1, 21, col)
	}
}

func thought(c *canvas, n int) {
	for i := 0; i < 3; i++ {
		col := uint8(cDotDim)
		if i < n {
			col = cDot
		}
		c.rect(9+i*3, 3, 10+i*3, 4, col)
	}
}

func sparkle(c *canvas, x, y int) { // pequena cruz branca
	c.rect(x, y, x, y, cDot)
	c.rect(x-1, y, x+1, y, cDot)
	c.rect(x, y-1, x, y+1, cDot)
}

func render(c *canvas) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, size, size), palette)
	for y := 0; y < grid; y++ {
		for x := 0; x < grid; x++ {
			r := image.Rect(x*scale, y*scale, (x+1)*scale, (y+1)*scale)
			draw.Draw(img, r, &image.Uniform{palette[c[y][x]]}, image.Point{}, draw.Src)
		}
	}
	return img
}

// ---- fonte 3x5 (cada glifo vira 6x10 px) e balao de fala ----

var font = map[rune][5]uint8{
	'P': {0b111, 0b101, 0b111, 0b100, 0b100},
	'R': {0b111, 0b101, 0b110, 0b101, 0b101},
	'O': {0b111, 0b101, 0b101, 0b101, 0b111},
	'N': {0b101, 0b111, 0b111, 0b111, 0b101},
	'T': {0b111, 0b010, 0b010, 0b010, 0b010},
	'!': {0b010, 0b010, 0b010, 0b000, 0b010},
	'E': {0b111, 0b100, 0b110, 0b100, 0b111},
	'C': {0b111, 0b100, 0b100, 0b100, 0b111},
	'I': {0b111, 0b010, 0b010, 0b010, 0b111},
	'S': {0b111, 0b100, 0b111, 0b001, 0b111},
	'D': {0b110, 0b101, 0b101, 0b101, 0b110},
	'V': {0b101, 0b101, 0b101, 0b101, 0b010},
	' ': {0, 0, 0, 0, 0},
}

func fillPx(img *image.Paletted, x0, y0, x1, y1 int, col uint8) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if image.Pt(x, y).In(img.Rect) {
				img.SetColorIndex(x, y, col)
			}
		}
	}
}

// bubble desenha um balao de fala no topo, centralizado, com a cauda apontando para o Clawd.
func bubble(img *image.Paletted, text string, bg uint8) { bubbleC(img, text, bg, cEye) }

// bubbleC e o balao com cores de fundo (bg) e de texto (fg) escolhidas pela paleta da pele.
func bubbleC(img *image.Paletted, text string, bg, fg uint8) {
	n := len([]rune(text))
	textW := n*8 - 2
	w, h := textW+16, 22
	x0, y0 := (size-w)/2, 8
	fillPx(img, x0, y0, x0+w-1, y0+h-1, bg)
	cx := size / 2
	for i := 0; i < 6; i++ { // cauda triangular
		fillPx(img, cx-(6-i), y0+h+i, cx+(6-i)-1, y0+h+i, bg)
	}
	tx, ty := x0+8, y0+6
	for _, r := range text {
		g, ok := font[r]
		if !ok {
			g = font[' ']
		}
		for row := 0; row < 5; row++ {
			for col := 0; col < 3; col++ {
				if g[row]&(1<<(2-col)) != 0 {
					fillPx(img, tx+col*2, ty+row*2, tx+col*2+1, ty+row*2+1, fg)
				}
			}
		}
		tx += 8
	}
}

type anim struct {
	frames []*image.Paletted
	delays []int
}

func (a *anim) add(img *image.Paletted, delay int) {
	a.frames = append(a.frames, img)
	a.delays = append(a.delays, delay)
}

// working: Clawd digitando, pensamentos acendendo e teclado.
func working() anim {
	type f struct {
		armL, armR int
		eyes       int
		key, dots  int
	}
	seq := []f{
		{0, 2, 0, 0, 0}, {2, 0, 0, 1, 1}, {0, 2, 0, 2, 2},
		{2, 0, 1, 3, 3}, {0, 2, 0, 4, 2}, {2, 0, 0, 5, 1},
	}
	var a anim
	for _, s := range seq {
		var c canvas
		thought(&c, s.dots)
		clawd(&c, pose{armL: s.armL, armR: s.armR, eyes: s.eyes})
		keyboard(&c, s.key)
		a.add(render(&c), 25)
	}
	return a
}

// done: Clawd pulando de alegria com bracos para cima, brilhos e balao "PRONTO!".
func done() anim {
	type f struct {
		dy, armL, armR int
		sp             int
	}
	seq := []f{
		{0, -3, -3, 0}, {-1, -4, -2, 1}, {0, -3, -3, 2}, {-1, -2, -4, 3},
	}
	spots := [][2]int{{3, 8}, {20, 9}, {4, 17}, {19, 18}}
	var a anim
	for _, s := range seq {
		var c canvas
		clawd(&c, pose{dy: s.dy + 2, armL: s.armL, armR: s.armR, eyes: 2})
		sparkle(&c, spots[s.sp][0], spots[s.sp][1])
		sparkle(&c, spots[(s.sp+2)%4][0], spots[(s.sp+2)%4][1])
		img := render(&c)
		bubble(img, "PRONTO!", cDot)
		a.add(img, 30)
	}
	return a
}

// needs: Clawd acenando com um braco e balao piscando "PRECISO DE VOCE".
func needs() anim {
	seq := []struct {
		armR  int
		alert bool
	}{{-4, true}, {-3, false}, {-4, true}, {-3, false}}
	var a anim
	for _, s := range seq {
		var c canvas
		clawd(&c, pose{dy: 2, armL: 0, armR: s.armR, eyes: 0})
		img := render(&c)
		bg := uint8(cDot)
		if s.alert {
			bg = cAlert
		}
		bubble(img, "PRECISO DE VOCE", bg)
		a.add(img, 35)
	}
	return a
}

func writeGIF(path string, a anim) {
	fh, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer fh.Close()
	if err := gif.EncodeAll(fh, &gif.GIF{Image: a.frames, Delay: a.delays, LoopCount: 0}); err != nil {
		log.Fatal(err)
	}
}

// writeSheet monta uma folha de contato PNG para conferir os quadros.
func writeSheet(path string, frames []*image.Paletted, cols int) {
	rows := (len(frames) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*size+(cols+1)*8, rows*size+(rows+1)*8))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.RGBA{60, 60, 60, 255}}, image.Point{}, draw.Src)
	for i, fr := range frames {
		x := 8 + (i%cols)*(size+8)
		y := 8 + (i/cols)*(size+8)
		draw.Draw(sheet, image.Rect(x, y, x+size, y+size), fr, image.Point{}, draw.Src)
	}
	fh, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer fh.Close()
	if err := png.Encode(fh, sheet); err != nil {
		log.Fatal(err)
	}
}

func main() {
	out := flag.String("out", "out", "pasta de saida")
	skin := flag.String("skin", "base", "pele: base | ninja")
	sheets := flag.Bool("sheets", false, "tambem gera folhas de contato PNG (previa dos quadros)")
	flag.Parse()
	gens := map[string]anim{"working": working(), "done": done(), "needs": needs()}
	switch *skin {
	case "base":
	case "ninja":
		gens = map[string]anim{"working": ninjaWorking(), "done": ninjaDone(), "needs": ninjaNeeds()}
		*out = filepath.Join(*out, "ninja")
	default:
		log.Fatalf("pele desconhecida: %s", *skin)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for name, a := range gens {
		writeGIF(filepath.Join(*out, name+".gif"), a)
		if *sheets {
			writeSheet(filepath.Join(*out, name+"-sheet.png"), a.frames, 6)
		}
		log.Printf("%s/%s: %d quadros %dx%d", *skin, name, len(a.frames), size, size)
	}
}
