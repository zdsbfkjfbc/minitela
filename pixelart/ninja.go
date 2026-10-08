// Pele "ninja" (nivel 10): Clawd com testeira e uma esfera de energia azul girando.
// Grade 64x64 de celulas de 3 px (192x192), mais fina que a do Clawd base, para animar de forma suave.
package main

import (
	"image"
	"image/color"
	"math"
)

const (
	ng     = 64
	nscale = 3
)

// Indices da paleta (16 cores = GIF de 4 bits).
const (
	nBg = iota
	nOrange
	nShade
	nEye
	nEyeHi
	nBand
	nPlate
	nWhisk
	nCore
	nSphLight
	nSphMid
	nSphDark
	nGround
	nBubble
	nAlert
	nSpark
)

var nPalette = color.Palette{
	nBg:       color.RGBA{0x14, 0x16, 0x1c, 0xff},
	nOrange:   color.RGBA{215, 119, 87, 0xff},
	nShade:    color.RGBA{176, 92, 66, 0xff},
	nEye:      color.RGBA{0x1e, 0x14, 0x14, 0xff},
	nEyeHi:    color.RGBA{0xff, 0xff, 0xff, 0xff},
	nBand:     color.RGBA{36, 66, 150, 0xff},
	nPlate:    color.RGBA{176, 186, 200, 0xff},
	nWhisk:    color.RGBA{120, 56, 40, 0xff},
	nCore:     color.RGBA{240, 252, 255, 0xff},
	nSphLight: color.RGBA{130, 205, 255, 0xff},
	nSphMid:   color.RGBA{50, 140, 240, 0xff},
	nSphDark:  color.RGBA{22, 80, 180, 0xff},
	nGround:   color.RGBA{28, 30, 38, 0xff},
	nBubble:   color.RGBA{0xee, 0xee, 0xf0, 0xff},
	nAlert:    color.RGBA{255, 208, 96, 0xff},
	nSpark:    color.RGBA{190, 235, 255, 0xff},
}

type ncanvas [ng][ng]uint8

func (c *ncanvas) rect(x0, y0, x1, y1 int, col uint8) { // inclusivo
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x >= 0 && x < ng && y >= 0 && y < ng {
				c[y][x] = col
			}
		}
	}
}

func (c *ncanvas) fill(col uint8) { c.rect(0, 0, ng-1, ng-1, col) }

func nrender(c *ncanvas) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, ng*nscale, ng*nscale), nPalette)
	for y := 0; y < ng; y++ {
		for x := 0; x < ng; x++ {
			v := c[y][x]
			if v == 0 {
				continue
			}
			for dy := 0; dy < nscale; dy++ {
				for dx := 0; dx < nscale; dx++ {
					img.SetColorIndex(x*nscale+dx, y*nscale+dy, v)
				}
			}
		}
	}
	return img
}

// sphere desenha a esfera de energia: espiral de 3 bracos que gira com phase (periodo 1).
func (c *ncanvas) sphere(cx, cy, r, phase float64) {
	for y := int(cy-r) - 1; y <= int(cy+r)+1; y++ {
		for x := int(cx-r) - 1; x <= int(cx+r)+1; x++ {
			if x < 0 || x >= ng || y < 0 || y >= ng {
				continue
			}
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy)
			if d > r {
				continue
			}
			v := math.Atan2(dy, dx)/(2*math.Pi)*3 + d/r*1.6 - phase
			v -= math.Floor(v)
			var col uint8
			switch {
			case d < r*0.26:
				col = nCore
			case d > r*0.9:
				col = nSphDark
			case v < 0.34:
				col = nSphLight
			case v < 0.68:
				col = nSphMid
			default:
				col = nSphDark
			}
			c[y][x] = col
		}
	}
}

// wisps desenha fagulhas de chakra em orbita (turn em [0,1): uma volta por ciclo).
func (c *ncanvas) wisps(cx, cy, r, turn float64, n int) {
	for i := 0; i < n; i++ {
		a := turn*2*math.Pi + float64(i)*2*math.Pi/float64(n)
		rr := r*1.35 + 2*math.Sin(turn*4*math.Pi+float64(i))
		x := int(math.Round(cx + math.Cos(a)*rr))
		y := int(math.Round(cy + math.Sin(a)*rr*0.85))
		c.rect(x, y, x+1, y+1, nSpark)
	}
}

func (c *ncanvas) ring(cx, cy, r, th float64, col uint8) {
	for y := 0; y < ng; y++ {
		for x := 0; x < ng; x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			if math.Abs(d-r) <= th/2 {
				c[y][x] = col
			}
		}
	}
}

func (c *ncanvas) cross(x, y int, col uint8) {
	c.rect(x-2, y, x+2, y, col)
	c.rect(x, y-2, x, y+2, col)
}

type nin struct {
	dy   int     // deslocamento vertical (balanco / pulo)
	arms string  // hold | down | cheer | wave | throw
	wave int     // deslocamento da mao no aceno
	eyes int     // 0 foco, 1 piscando, 2 feliz
	tail float64 // fase das pontas da testeira
}

func (c *ncanvas) ninja(p nin) {
	dy := p.dy
	c.rect(16, 58, 47, 59, nGround) // sombra no chao

	// bracos (por tras do corpo)
	switch p.arms {
	case "hold":
		c.rect(14, 20+dy, 17, 36+dy, nOrange)
		c.rect(18, 18+dy, 21, 22+dy, nOrange)
		c.rect(46, 20+dy, 49, 36+dy, nOrange)
		c.rect(42, 18+dy, 45, 22+dy, nOrange)
	case "throw":
		c.rect(14, 26+dy, 17, 40+dy, nOrange)
		c.rect(46, 26+dy, 49, 40+dy, nOrange)
	case "cheer":
		c.rect(14, 30+dy, 17, 36+dy, nOrange)
		c.rect(11, 14+dy, 14, 31+dy, nOrange)
		c.rect(46, 30+dy, 49, 36+dy, nOrange)
		c.rect(49, 14+dy, 52, 31+dy, nOrange)
	case "wave":
		c.rect(14, 34+dy, 17, 46+dy, nOrange)
		c.rect(46, 30+dy, 49, 36+dy, nOrange)
		c.rect(50+p.wave, 14+dy, 53+p.wave, 31+dy, nOrange)
	default: // down
		c.rect(14, 34+dy, 17, 46+dy, nOrange)
		c.rect(46, 34+dy, 49, 46+dy, nOrange)
	}

	// pernas
	for _, x := range []int{20, 26, 34, 40} {
		c.rect(x, 48+dy, x+3, 57+dy, nShade)
	}
	// corpo
	c.rect(18, 30+dy, 45, 47+dy, nOrange)
	c.rect(18, 44+dy, 45, 47+dy, nShade)

	// testeira com placa e pontas ondulando
	c.rect(18, 30+dy, 45, 34+dy, nBand)
	c.rect(27, 30+dy, 36, 34+dy, nPlate)
	c.rect(30, 31+dy, 33, 33+dy, nBand)
	c.rect(31, 32+dy, 32, 32+dy, nPlate)
	for i := 0; i < 8; i++ {
		off := int(math.Round(math.Sin(p.tail+float64(i)*0.55) * float64(i) / 3))
		c.rect(46+i, 31+dy+off, 46+i, 32+dy+off, nBand)
		c.rect(46+i, 34+dy+off, 46+i, 35+dy+off, nBand)
	}

	// marquinhas no rosto
	for k := 0; k < 3; k++ {
		y := 38 + k*2 + dy
		c.rect(19, y, 22, y, nWhisk)
		c.rect(41, y, 44, y, nWhisk)
	}

	// olhos
	switch p.eyes {
	case 1:
		c.rect(24, 40+dy, 27, 40+dy, nEye)
		c.rect(36, 40+dy, 39, 40+dy, nEye)
	case 2:
		for _, x := range []int{24, 36} {
			c.rect(x, 40+dy, x, 41+dy, nEye)
			c.rect(x+1, 38+dy, x+2, 39+dy, nEye)
			c.rect(x+3, 40+dy, x+3, 41+dy, nEye)
		}
	default:
		c.rect(24, 36+dy, 27, 41+dy, nEye)
		c.rect(36, 36+dy, 39, 41+dy, nEye)
		c.rect(25, 37+dy, 25, 37+dy, nEyeHi)
		c.rect(37, 37+dy, 37, 37+dy, nEyeHi)
	}
}

// ninjaWorking: carrega a esfera entre as maos, que gira e pulsa (ciclo de 12 quadros, sem emenda).
func ninjaWorking() anim {
	var a anim
	const n = 12
	for i := 0; i < n; i++ {
		t := float64(i) / n
		bob := int(math.Round(math.Sin(2 * math.Pi * t)))
		eyes := 0
		if i == 7 {
			eyes = 1
		}
		var c ncanvas
		c.ninja(nin{dy: bob, arms: "hold", eyes: eyes, tail: 2 * math.Pi * t})
		r := 10.5 + math.Sin(2*math.Pi*t)
		cy := 15.0 + float64(bob)
		c.sphere(32, cy, r, 2*t)
		c.wisps(32, cy, r, t, 8)
		a.add(nrender(&c), 8)
	}
	return a
}

// ninjaDone: carrega, arremessa a esfera contra a tela, clarao, ondas e comemora com o balao.
func ninjaDone() anim {
	var a anim
	for i := 0; i < 4; i++ { // carga
		t := float64(i) / 4
		var c ncanvas
		c.ninja(nin{arms: "hold", tail: 2 * math.Pi * t})
		r := 10.5 + float64(i)*0.6
		c.sphere(32, 15, r, 2*t)
		c.wisps(32, 15, r, t, 10)
		a.add(nrender(&c), 8)
	}
	radii := []float64{13, 18, 26, 38, 54}
	cys := []float64{16, 18, 21, 25, 30}
	for i, r := range radii { // arremesso: a esfera cresce vindo para a tela
		var c ncanvas
		c.ninja(nin{arms: "throw", tail: float64(i)})
		c.sphere(32, cys[i], r, 2*float64(i)/5)
		a.add(nrender(&c), 6)
	}
	var f ncanvas // clarao
	f.fill(nCore)
	a.add(nrender(&f), 5)
	var g ncanvas
	g.fill(nSphLight)
	a.add(nrender(&g), 4)
	for i, rr := range []float64{10, 22, 36} { // ondas de impacto
		var c ncanvas
		c.ring(32, 30, rr, 3, nSphMid)
		c.ring(32, 30, rr-3, 2, nSpark)
		c.ninja(nin{arms: "cheer", eyes: 2, dy: -1 * (i % 2)})
		a.add(nrender(&c), 6)
	}
	jumps := []int{-2, -4, -5, -3, -1, 0, 0, 0}
	spots := [][2]int{{8, 22}, {54, 24}, {10, 46}, {53, 44}}
	for i, dy := range jumps { // comemoracao com o balao
		var c ncanvas
		c.ninja(nin{arms: "cheer", eyes: 2, dy: dy, tail: float64(i)})
		c.cross(spots[i%4][0], spots[i%4][1], nCore)
		c.cross(spots[(i+2)%4][0], spots[(i+2)%4][1], nSpark)
		img := nrender(&c)
		bubbleC(img, "PRONTO!", nBubble, nEye)
		d := 9
		if i == len(jumps)-1 {
			d = 30
		}
		a.add(img, d)
	}
	return a
}

// ninjaNeeds: acena com uma esfera pequena pulsando sobre a cabeca e o balao piscando.
func ninjaNeeds() anim {
	var a anim
	const n = 8
	for i := 0; i < n; i++ {
		t := float64(i) / n
		w := int(math.Round(math.Sin(2*math.Pi*t*2) * 2))
		eyes := 0
		if i == 5 {
			eyes = 1
		}
		var c ncanvas
		c.ninja(nin{arms: "wave", wave: w, eyes: eyes, tail: 2 * math.Pi * t})
		c.sphere(32, 20, 5.5+0.8*math.Sin(2*math.Pi*t), 2*t)
		img := nrender(&c)
		bg := uint8(nBubble)
		if (i/2)%2 == 0 {
			bg = nAlert
		}
		bubbleC(img, "PRECISO DE VOCE", bg, nEye)
		a.add(img, 10)
	}
	return a
}
