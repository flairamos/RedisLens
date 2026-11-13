// genlogo 程序化生成 RedisLens 品牌资产：
//   - build/appicon.png          (512x512 应用图标)
//   - build/windows/icon.ico    (16-256 多尺寸)
//   - frontend/src/assets/logo.svg (界面矢量标识)
//
// 设计：深色圆角方形底 + 白色描边放大镜 + 玻璃内的 Redis 三层菱形堆叠，
// 手柄使用 Redis 红 #DC382D。纯标准库绘制，4x 超采样抗锯齿。
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const (
	size = 512
	sub  = 4 // 超采样倍数
)

var (
	red    = [3]float64{0xDC / 255.0, 0x38 / 255.0, 0x2D / 255.0}
	redMid = [3]float64{0xC0 / 255.0, 0x2C / 255.0, 0x22 / 255.0}
	redHi  = [3]float64{0xE8 / 255.0, 0x50 / 255.0, 0x44 / 255.0}
)

func main() {
	outDir := "."
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	img := render(size)
	if err := savePNG(filepath.Join(outDir, "build", "appicon.png"), img); err != nil {
		panic(err)
	}
	// ICO 多尺寸
	sizes := []int{256, 128, 64, 48, 32, 24, 16}
	if err := saveICO(filepath.Join(outDir, "build", "windows", "icon.ico"), img, sizes); err != nil {
		panic(err)
	}
	// 前端大图（用于窗口内展示）
	if err := savePNG(filepath.Join(outDir, "frontend", "src", "assets", "logo.png"), img); err != nil {
		panic(err)
	}
	fmt.Println("logo assets generated")
}

// render 渲染 512px 图标。
func render(sz int) image.Image {
	full := sz * sub
	canvas := image.NewRGBA(image.Rect(0, 0, full, full))
	u := float64(full) // 归一化单位

	for py := 0; py < full; py++ {
		for px := 0; px < full; px++ {
			// 像素中心
			x := (float64(px) + 0.5) / u
			y := (float64(py) + 0.5) / u
			c := sample(x, y)
			canvas.Set(px, py, c)
		}
	}
	// 下采样
	out := image.NewRGBA(image.Rect(0, 0, sz, sz))
	for py := 0; py < sz; py++ {
		for px := 0; px < sz; px++ {
			var r, g, b, a float64
			for sy := 0; sy < sub; sy++ {
				for sx := 0; sx < sub; sx++ {
					c := canvas.At(px*sub+sx, py*sub+sy).(color.RGBA)
					r += float64(c.R)
					g += float64(c.G)
					b += float64(c.B)
					a += float64(c.A)
				}
			}
			n := float64(sub * sub)
			out.SetRGBA(px, py, color.RGBA{
				R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: uint8(a / n),
			})
		}
	}
	return out
}

// sample 单点取色。
func sample(x, y float64) color.RGBA {
	// 背景圆角方形渐变
	bg := roundedRect(x, y, 0.02, 0.02, 0.96, 0.96, 0.22)
	if bg <= 0 {
		return color.RGBA{0, 0, 0, 0}
	}
	// 垂直渐变 #272C36 -> #1B1F26
	t := clamp01((y - 0.02) / 0.96)
	base := lerpColor([3]float64{0x27 / 255.0, 0x2C / 255.0, 0x36 / 255.0}, [3]float64{0x1B / 255.0, 0x1F / 255.0, 0x26 / 255.0}, t)

	// 红色柔光（玻璃后方）
	glow := circle(x, y, 0.46, 0.45, 0.30)
	base = mixColor(base, red, 0.16*glow)

	// 玻璃内部
	glass := circle(x, y, 0.46, 0.45, 0.255)
	glassColor := lerpColor([3]float64{0x34 / 255.0, 0x3A / 255.0, 0x46 / 255.0}, [3]float64{0x2A / 255.0, 0x30 / 255.0, 0x3A / 255.0}, t)
	if glass > 0 {
		base = mixColor(base, glassColor, glass)
	}

	// 菱形堆叠（三层）
	base = drawStack(base, x, y)

	// 镜柄（45° 手柄，先画避免被镜圈覆盖）
	base = drawHandle(base, x, y)

	// 镜圈（白色）
	ringDist := ringSD(x, y, 0.46, 0.45, 0.285, 0.038)
	if ringDist > 0 {
		base = mixColor(base, [3]float64{0.96, 0.97, 0.99}, ringDist)
	}
	out := toRGBA(base)
	out.A = uint8(clamp01(bg) * 255)
	return out
}

// drawStack 绘制三层菱形（上小下大，带描边阴影感）。
func drawStack(base [3]float64, x, y float64) [3]float64 {
	layers := []struct {
		cy  float64
		c   [3]float64
		hw  float64 // 半宽
		hh  float64 // 半高
	}{
		{0.535, redHi, 0.105, 0.052},
		{0.462, red, 0.105, 0.052},
		{0.389, redMid, 0.105, 0.052},
	}
	// 先画底层，再画上层覆盖（上层在后覆盖下层 => 逆序绘制，顶层最后）
	// 我们按从下到上绘制：底部 layer[0] 先画
	for i := len(layers) - 1; i >= 0; i-- {
		l := layers[i]
		d := diamondSD(x, y, 0.46, l.cy, l.hw, l.hh)
		if d > 0 {
			// 顶部高光：菱形上半部分微亮
			hi := clamp01((l.cy - y) / (l.hh * 2))
			col := lerpColor(l.c, [3]float64{1, 1, 1}, 0.18*hi)
			// 底部阴影
			sh := clamp01((y - l.cy) / (l.hh * 2))
			col = lerpColor(col, [3]float64{0.35, 0.08, 0.06}, 0.35*sh)
			base = mixColor(base, col, d)
		}
	}
	return base
}

// drawHandle 绘制镜柄（带圆角端帽的粗线段）。
func drawHandle(base [3]float64, x, y float64) [3]float64 {
	ax, ay := 0.655, 0.645
	bx, by := 0.79, 0.78
	w := 0.052
	d := capsuleSD(x, y, ax, ay, bx, by, w)
	if d > 0 {
		base = mixColor(base, red, d)
	}
	return base
}

// ---------- 几何 SDF ----------

// roundedRect 圆角矩形覆盖因子（标准 SDF，1 内部 / 0 外部，边缘抗锯齿）。
func roundedRect(x, y, x0, y0, x1, y1, r float64) float64 {
	cx := clamp(x, x0+r, x1-r)
	cy := clamp(y, y0+r, y1-r)
	dx := x - cx
	dy := y - cy
	d := sqrtf(dx*dx+dy*dy) - r // 负值在内部
	aa := 0.0022
	return clamp01((-d+aa) / (2 * aa))
}

// circle 覆盖因子（0-1）。
func circle(x, y, cx, cy, r float64) float64 {
	d := dist(x, y, cx, cy)
	return clamp01((r - d) / 0.0025)
}

// ringSD 圆环覆盖因子。
func ringSD(x, y, cx, cy, r, w float64) float64 {
	d := abs(dist(x, y, cx, cy) - r)
	return clamp01((w - d) / 0.0025)
}

// diamondSD 菱形覆盖因子。
func diamondSD(x, y, cx, cy, hw, hh float64) float64 {
	// |x-cx|/hw + |y-cy|/hh <= 1
	v := abs(x-cx)/hw + abs(y-cy)/hh
	return clamp01((1 - v) / 0.01)
}

// capsuleSD 胶囊（线段+圆帽）覆盖因子。
func capsuleSD(x, y, ax, ay, bx, by, w float64) float64 {
	pax, pay := x-ax, y-ay
	bax, bay := bx-ax, by-ay
	h := clamp01(dot(pax, pay, bax, bay) / dot(bax, bay, bax, bay))
	dx := pax - h*bax
	dy := pay - h*bay
	d := sqrtf(dx*dx + dy*dy)
	return clamp01((w - d) / 0.003)
}

// ---------- 小工具 ----------

func dist(x, y, cx, cy float64) float64 {
	dx, dy := x-cx, y-cy
	return sqrtf(dx*dx + dy*dy)
}

func dot(ax, ay, bx, by float64) float64 { return ax*bx + ay*by }
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func sqrtf(v float64) float64 {
	if v <= 0 {
		return 0
	}
	// 牛顿迭代
	z := v
	for i := 0; i < 24; i++ {
		z = 0.5 * (z + v/z)
	}
	return z
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float64) float64 { return clamp(v, 0, 1) }

func lerpColor(a, b [3]float64, t float64) [3]float64 {
	t = clamp01(t)
	return [3]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func mixColor(base, over [3]float64, alpha float64) [3]float64 {
	alpha = clamp01(alpha)
	return [3]float64{
		base[0]*(1-alpha) + over[0]*alpha,
		base[1]*(1-alpha) + over[1]*alpha,
		base[2]*(1-alpha) + over[2]*alpha,
	}
}

func toRGBA(c [3]float64) color.RGBA {
	return color.RGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: 255,
	}
}

func savePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G703 -- 开发期工具
		return err
	}
	f, err := os.Create(path) //#nosec G304,G703 -- 开发期图标生成工具，路径由构建脚本传入
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck
	return png.Encode(f, img)
}

// saveICO 将多尺寸 PNG 打包为 ICO。
// 尺寸取自固定表 sizes（16-256），转换不会溢出。开发期构建工具，非产品代码。
func saveICO( /* #nosec G115,G703 -- 有界常量转换与固定输出路径 */ path string, src image.Image, sizes []int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G703 -- 开发期工具
		return err
	}
	type entry struct {
		w, h  uint16
		names int32
	}
	var entries []entry
	var pngs [][]byte
	for _, s := range sizes {
		img := resize(src, s)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		pngs = append(pngs, buf.Bytes())
		entries = append(entries, entry{w: uint16(s), h: uint16(s), names: int32(len(pngs) - 1)})
	}
	var out bytes.Buffer
	// ICONDIR
	out.Write([]byte{0, 0})       // reserved
	out.Write([]byte{1, 0})       // type: icon
	out.Write([]byte{byte(len(entries)), 0})
	offset := 6 + 16*len(entries)
	for i, e := range entries {
		dim := e.w
		if dim == 256 {
			dim = 0
		}
		out.Write([]byte{byte(dim), byte(dim)}) // w, h
		out.Write([]byte{0, 0})                  // palette
		out.Write([]byte{1, 0})                  // planes
		out.Write([]byte{32, 0})                 // bpp
		size := uint32(len(pngs[i]))
		out.Write([]byte{byte(size), byte(size >> 8), byte(size >> 16), byte(size >> 24)})
		out.Write([]byte{
			byte(offset), byte(offset >> 8), byte(offset >> 16), byte(offset >> 24),
		})
		offset += int(size)
	}
	for _, p := range pngs {
		out.Write(p)
	}
	return os.WriteFile(path, out.Bytes(), 0o600)
}

// resize 简单盒式下采样（对 512 源图缩小到目标尺寸）。
func resize(src image.Image, size int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// 最近邻 + 平均：按源图对应区域平均
			sx0 := x * sw / size
			sx1 := (x + 1) * sw / size
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			sy0 := y * sh / size
			sy1 := (y + 1) * sh / size
			if sy1 <= sy0 {
				sy1 = sy0 + 1
			}
			var r, g, bl, a float64
			n := 0
			for sy := sy0; sy < sy1 && sy < sh; sy++ {
				for sx := sx0; sx < sx1 && sx < sw; sx++ {
					c := src.At(b.Min.X+sx, b.Min.Y+sy).(color.RGBA)
					r += float64(c.R)
					g += float64(c.G)
					bl += float64(c.B)
					a += float64(c.A)
					n++
				}
			}
			if n == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r / float64(n)), G: uint8(g / float64(n)),
				B: uint8(bl / float64(n)), A: uint8(a / float64(n)),
			})
		}
	}
	return dst
}
