package mediacheck

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sort"
)

// captionRegions locates coherent character-sized components in both text
// polarities. Whole background blocks and isolated decoration are excluded.
// Recognition still uses the original pixels and the same confidence rules.
func captionRegions(frame []byte) [][]byte {
	picture, err := png.Decode(bytes.NewReader(frame))
	if err != nil {
		return nil
	}
	b := picture.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 100 || h < 20 || w*h > 4_000_000 {
		return nil
	}
	type band struct {
		box     image.Rectangle
		anchor  image.Rectangle
		count   int
		heights []int
	}
	var candidates []band
	for _, light := range []bool{false, true} {
		pixels := make([]bool, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				value := color.GrayModel.Convert(picture.At(x+b.Min.X, y+b.Min.Y)).(color.Gray).Y
				pixels[y*w+x] = light && value >= 240 || !light && value <= 64
			}
		}
		var bands []band
		for start := range pixels {
			if !pixels[start] {
				continue
			}
			pixels[start] = false
			queue := []int{start}
			x, y := start%w, start/w
			rect := image.Rect(x, y, x+1, y+1)
			area := 0
			for len(queue) > 0 {
				p := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				px, py := p%w, p/w
				area++
				rect = rect.Union(image.Rect(px, py, px+1, py+1))
				for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					nx, ny := px+d[0], py+d[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h && pixels[ny*w+nx] {
						pixels[ny*w+nx] = false
						queue = append(queue, ny*w+nx)
					}
				}
			}
			if area < 24 || rect.Dy() < 18 || rect.Dy() > 100 || rect.Dx() < 4 || rect.Dx() > 140 {
				continue
			}
			found := false
			for i := range bands {
				old := bands[i].anchor
				centerDelta := (old.Min.Y + old.Max.Y) - (rect.Min.Y + rect.Max.Y)
				if centerDelta < 0 {
					centerDelta = -centerDelta
				}
				// Anchor alignment and glyph size cannot drift as background
				// components are added to a row's growing bounding rectangle.
				if min(old.Dy(), rect.Dy())*5 >= max(old.Dy(), rect.Dy())*4 && centerDelta*2 <= min(old.Dy(), rect.Dy()) {
					bands[i].box = bands[i].box.Union(rect)
					bands[i].count++
					bands[i].heights = append(bands[i].heights, rect.Dy())
					found = true
					break
				}
			}
			if !found {
				bands = append(bands, band{box: rect, anchor: rect, count: 1, heights: []int{rect.Dy()}})
			}
		}
		for _, v := range bands {
			if v.count >= 4 && v.box.Dx() >= v.box.Dy()*3 && v.box.Dx() < w*9/10 && v.box.Dy() <= 110 {
				candidates = append(candidates, v)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].count > candidates[j].count })
	// Bilingual captions must not merge a smaller Japanese row into Chinese
	// script evidence. Use character heights, not a noisy row's bounding box.
	largest := 0
	for _, v := range candidates {
		sort.Ints(v.heights)
		largest = max(largest, v.heights[len(v.heights)/2])
	}
	var output [][]byte
	var boxes []image.Rectangle
	for _, v := range candidates {
		if v.heights[len(v.heights)/2]*5 < largest*4 {
			continue
		}
		r := v.box.Inset(-8).Intersect(image.Rect(0, 0, w, h))
		duplicate := false
		for _, old := range boxes {
			intersection := old.Intersect(r)
			if intersection.Dx()*intersection.Dy()*2 >= r.Dx()*r.Dy() {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		crop := image.NewRGBA(image.Rect(0, 0, r.Dx()+16, r.Dy()+16))
		draw.Draw(crop, crop.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(crop, image.Rect(8, 8, r.Dx()+8, r.Dy()+8), picture, r.Min.Add(b.Min), draw.Src)
		var encoded bytes.Buffer
		if png.Encode(&encoded, crop) == nil {
			output = append(output, encoded.Bytes())
			boxes = append(boxes, r)
		}
		if len(output) == 2 {
			break
		}
	}
	return output
}

// outlinedCaption removes the light background connected to the crop border.
// The white strokes enclosed by dark outlines remain. No glyph is synthesized.
func outlinedCaption(frame []byte) ([]byte, error) {
	picture, err := png.Decode(bytes.NewReader(frame))
	if err != nil {
		return nil, err
	}
	b := picture.Bounds()
	w, h := b.Dx(), b.Dy()
	bright := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bright[y*w+x] = color.GrayModel.Convert(picture.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y >= 210
		}
	}
	queue := []int{}
	push := func(x, y int) {
		p := y*w + x
		if bright[p] {
			bright[p] = false
			queue = append(queue, p)
		}
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for len(queue) > 0 {
		p := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x, y := p%w, p/w
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			nx, ny := x+d[0], y+d[1]
			if nx >= 0 && nx < w && ny >= 0 && ny < h {
				push(nx, ny)
			}
		}
	}
	mask := image.NewGray(image.Rect(0, 0, w, h))
	for i, on := range bright {
		shade := uint8(255)
		if on {
			shade = 0
		}
		mask.Pix[i] = shade
	}
	var output bytes.Buffer
	err = png.Encode(&output, mask)
	return output.Bytes(), err
}
