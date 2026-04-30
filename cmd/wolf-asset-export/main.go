package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Distortions81/impsynth"

	"gd-wolf/internal/wl6"
)

const (
	mapPlaneScale     = 4
	mapOverviewTilePx = 16
	musicRenderRate   = 44100
	musicTickRate     = 700
)

func main() {
	dataDir := ""
	outDir := "exported-assets"
	imageLayout := "all"
	magnify := "1"
	rest, err := parseToolArgs(os.Args[1:], map[string]*string{
		"-data":          &dataDir,
		"--data":         &dataDir,
		"-out":           &outDir,
		"--out":          &outDir,
		"-image-layout":  &imageLayout,
		"--image-layout": &imageLayout,
		"-magnify":       &magnify,
		"--magnify":      &magnify,
	})
	if err != nil {
		if err == flag.ErrHelp {
			usage()
			os.Exit(2)
		}
		fatal(err)
	}
	if len(rest) > 1 {
		usage()
		os.Exit(2)
	}

	files, src, err := openFiles(dataDir)
	if err != nil {
		fatal(err)
	}
	target := "all"
	if len(rest) == 1 {
		target = rest[0]
	}
	magnifyScale, err := strconv.Atoi(magnify)
	if err != nil || magnifyScale < 1 {
		fatal(fmt.Errorf("invalid magnify value %q", magnify))
	}
	if err := exportTarget(files, src, outDir, target, imageLayout, magnifyScale); err != nil {
		fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: wolf-asset-export [-data path] [-out dir] [target]")
	fmt.Fprintln(os.Stderr, "targets: all, pictures, walls, sprites, maps, music, sounds")
	fmt.Fprintln(os.Stderr, "  -data path")
	fmt.Fprintln(os.Stderr, "        path to Wolfenstein 3D data directory")
	fmt.Fprintln(os.Stderr, "  -out dir")
	fmt.Fprintln(os.Stderr, "        output directory (default \"exported-assets\")")
	fmt.Fprintln(os.Stderr, "  -image-layout files|sheets|all")
	fmt.Fprintln(os.Stderr, "        image export layout for pictures, walls, and sprites (default \"all\")")
	fmt.Fprintln(os.Stderr, "  -magnify N")
	fmt.Fprintln(os.Stderr, "        integer nearest-neighbor scale for exported images (default 1)")
	fmt.Fprintln(os.Stderr, "If [target] is omitted, exports everything.")
}

func parseToolArgs(args []string, stringFlags map[string]*string) ([]string, error) {
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help", "help":
			return nil, flag.ErrHelp
		}
		if dst, ok := stringFlags[arg]; ok {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}
			*dst = args[i+1]
			i++
			continue
		}
		rest = append(rest, arg)
	}
	return rest, nil
}

func openFiles(dataDir string) (*wl6.Files, string, error) {
	if dataDir == "" {
		return wl6.OpenDefault()
	}
	files, err := wl6.Open(dataDir)
	return files, dataDir, err
}

func exportTarget(files *wl6.Files, src, outDir, target, imageLayout string, magnifyScale int) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	switch imageLayout {
	case "", "all", "files", "sheets":
	default:
		return fmt.Errorf("unsupported image layout %q", imageLayout)
	}
	if target == "all" {
		if err := writeMetadata(files, src, outDir); err != nil {
			return err
		}
		for _, step := range []func(*wl6.Files, string) error{
			func(files *wl6.Files, outDir string) error {
				return exportPictures(files, outDir, imageLayout, magnifyScale)
			},
			func(files *wl6.Files, outDir string) error {
				return exportWalls(files, outDir, imageLayout, magnifyScale)
			},
			func(files *wl6.Files, outDir string) error {
				return exportSprites(files, outDir, imageLayout, magnifyScale)
			},
			func(files *wl6.Files, outDir string) error { return exportMaps(files, outDir, magnifyScale) },
			exportMusic,
			exportSounds,
		} {
			if err := step(files, outDir); err != nil {
				return err
			}
		}
		return nil
	}

	switch target {
	case "pictures":
		return exportPictures(files, outDir, imageLayout, magnifyScale)
	case "walls":
		return exportWalls(files, outDir, imageLayout, magnifyScale)
	case "sprites":
		return exportSprites(files, outDir, imageLayout, magnifyScale)
	case "maps":
		return exportMaps(files, outDir, magnifyScale)
	case "music":
		return exportMusic(files, outDir)
	case "sounds":
		return exportSounds(files, outDir)
	default:
		return fmt.Errorf("unknown export target %q", target)
	}
}

func writeMetadata(files *wl6.Files, src, outDir string) error {
	content := fmt.Sprintf("variant=%s\next=%s\nsource=%s\n", files.Variant.Name, files.Variant.Ext, src)
	path := filepath.Join(outDir, "metadata.txt")
	return os.WriteFile(path, []byte(content), 0o644)
}

func exportPictures(files *wl6.Files, outDir, imageLayout string, magnifyScale int) error {
	start, end, err := files.PictureChunkRange()
	if err != nil {
		return fmt.Errorf("picture chunk range: %w", err)
	}
	dir := filepath.Join(outDir, "pictures")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	walls, err := files.LoadWallSet()
	if err != nil {
		return fmt.Errorf("load wall set for picture palette: %w", err)
	}
	entries := make([]sheetEntry, 0, end-start)
	for chunk := start; chunk < end; chunk++ {
		pic, err := files.LoadPicture(chunk)
		if err != nil {
			return fmt.Errorf("load picture chunk %d: %w", chunk, err)
		}
		if pictureIsPlaceholder(pic) {
			continue
		}
		img := pic.RGBA(walls.Palette)
		entries = append(entries, sheetEntry{
			Name:  fmt.Sprintf("chunk-%03d", chunk),
			Image: magnifyImage(img, magnifyScale),
		})
		if imageLayout == "all" || imageLayout == "files" || imageLayout == "" {
			name := filepath.Join(dir, fmt.Sprintf("chunk-%03d.png", chunk))
			if err := writePNG(name, img, magnifyScale); err != nil {
				return err
			}
		}
	}
	if imageLayout == "all" || imageLayout == "sheets" || imageLayout == "" {
		if err := writeSheetBundle(filepath.Join(dir, "sheet"), entries); err != nil {
			return err
		}
	}
	return nil
}

func pictureIsPlaceholder(pic *wl6.Picture) bool {
	if pic == nil || len(pic.Data) == 0 {
		return true
	}
	for _, px := range pic.Data {
		if px != 0 {
			return false
		}
	}
	return true
}

func exportWalls(files *wl6.Files, outDir, imageLayout string, magnifyScale int) error {
	set, err := files.LoadWallSet()
	if err != nil {
		return fmt.Errorf("load wall set: %w", err)
	}
	baseDir := filepath.Join(outDir, "walls")
	allDir := filepath.Join(baseDir, "pages")
	hDir := filepath.Join(baseDir, "horizontal")
	for _, dir := range []string{allDir, hDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	allEntries := make([]sheetEntry, 0, len(set.AllPages))
	hEntries := make([]sheetEntry, 0, len(set.Horizontal))
	for _, tex := range set.AllPages {
		if tex.Empty() {
			continue
		}
		if !isExportedHorizontalWallPage(tex.Page) && isPairedWallPage(tex.Page) {
			continue
		}
		img := wallTextureImage(tex)
		allEntries = append(allEntries, sheetEntry{
			Name:  fmt.Sprintf("page-%03d", tex.Page),
			Image: magnifyImage(img, magnifyScale),
		})
		if imageLayout == "all" || imageLayout == "files" || imageLayout == "" {
			path := filepath.Join(allDir, fmt.Sprintf("page-%03d.png", tex.Page))
			if err := writePNG(path, img, magnifyScale); err != nil {
				return err
			}
		}
	}
	for tile := 1; tile < len(set.Horizontal); tile++ {
		if !set.Horizontal[tile].Empty() {
			img := wallTextureImage(set.Horizontal[tile])
			hEntries = append(hEntries, sheetEntry{
				Name:  fmt.Sprintf("tile-%02d", tile),
				Image: magnifyImage(img, magnifyScale),
			})
			if imageLayout == "all" || imageLayout == "files" || imageLayout == "" {
				if err := writePNG(filepath.Join(hDir, fmt.Sprintf("tile-%02d.png", tile)), img, magnifyScale); err != nil {
					return err
				}
			}
		}
	}
	if imageLayout == "all" || imageLayout == "sheets" || imageLayout == "" {
		if err := writeSheetBundle(filepath.Join(allDir, "sheet"), allEntries); err != nil {
			return err
		}
		if err := writeSheetBundle(filepath.Join(hDir, "sheet"), hEntries); err != nil {
			return err
		}
	}
	return nil
}

func pairedWallPageCount() int {
	return (64 - 1) * 2
}

func isPairedWallPage(page int) bool {
	return page >= 0 && page < pairedWallPageCount()
}

func isExportedHorizontalWallPage(page int) bool {
	return isPairedWallPage(page) && page%2 == 0
}

func exportSprites(files *wl6.Files, outDir, imageLayout string, magnifyScale int) error {
	set, err := files.LoadSpriteSet()
	if err != nil {
		return fmt.Errorf("load sprite set: %w", err)
	}
	dir := filepath.Join(outDir, "sprites")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries := make([]sheetEntry, 0, len(set.Pages))
	for i, spr := range set.Pages {
		if spr.Width <= 0 || spr.Height <= 0 || len(spr.Pixels) == 0 {
			continue
		}
		img := spriteImage(spr)
		entries = append(entries, sheetEntry{
			Name:  fmt.Sprintf("shape-%03d", i),
			Image: magnifyImage(img, magnifyScale),
		})
		if imageLayout == "all" || imageLayout == "files" || imageLayout == "" {
			if err := writePNG(filepath.Join(dir, fmt.Sprintf("shape-%03d.png", i)), img, magnifyScale); err != nil {
				return err
			}
		}
	}
	if imageLayout == "all" || imageLayout == "sheets" || imageLayout == "" {
		if err := writeSheetBundle(filepath.Join(dir, "sheet"), entries); err != nil {
			return err
		}
	}
	return nil
}

func exportMaps(files *wl6.Files, outDir string, magnifyScale int) error {
	summaries, err := files.Maps()
	if err != nil {
		return fmt.Errorf("list maps: %w", err)
	}
	walls, err := files.LoadWallSet()
	if err != nil {
		return fmt.Errorf("load wall set for maps: %w", err)
	}
	baseDir := filepath.Join(outDir, "maps")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	for _, summary := range summaries {
		data, err := files.LoadMap(summary.Index)
		if err != nil {
			return fmt.Errorf("load map %d: %w", summary.Index, err)
		}
		level := data.Level()
		mapDir := filepath.Join(baseDir, fmt.Sprintf("%02d-%s", summary.Index, sanitizeName(summary.Name)))
		if err := os.MkdirAll(mapDir, 0o755); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(mapDir, "plane0-wall.png"), planeImage(data, 0, mapPlaneScale), magnifyScale); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(mapDir, "plane1-info.png"), planeImage(data, 1, mapPlaneScale), magnifyScale); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(mapDir, "overview.png"), mapOverviewImage(level, walls), magnifyScale); err != nil {
			return err
		}
	}
	return nil
}

func exportMusic(files *wl6.Files, outDir string) error {
	dir := filepath.Join(outDir, "music")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for song := 0; song < files.Variant.MusicCount; song++ {
		chunk, err := files.LoadMusicChunk(song)
		if err != nil {
			if strings.Contains(err.Error(), "unavailable") {
				continue
			}
			return fmt.Errorf("load music chunk %d: %w", song, err)
		}
		name := fmt.Sprintf("song-%02d", song)
		if chunk.Name != "" {
			name += "-" + sanitizeName(chunk.Name)
		}
		pcm := renderMusicChunkPCM(chunk, musicRenderRate)
		if err := os.WriteFile(filepath.Join(dir, name+".wav"), wavBytes(pcm, musicRenderRate), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func exportSounds(files *wl6.Files, outDir string) error {
	set, err := files.LoadDigitizedSounds()
	if err != nil {
		return fmt.Errorf("load digitized sounds: %w", err)
	}
	dir := filepath.Join(outDir, "sounds")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, sample := range set.Samples {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("sound-%03d.wav", i)), wavBytes(sample, set.SampleRate), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func wallTextureImage(tex wl6.WallTexture) image.Image {
	width, height := tex.Size()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rgba := tex.Pixel(x, y)
			i := img.PixOffset(x, y)
			img.Pix[i] = byte(rgba >> 24)
			img.Pix[i+1] = byte(rgba >> 16)
			img.Pix[i+2] = byte(rgba >> 8)
			img.Pix[i+3] = byte(rgba)
		}
	}
	return img
}

func spriteImage(spr wl6.Sprite) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, spr.Width, spr.Height))
	for y := 0; y < spr.Height; y++ {
		for x := 0; x < spr.Width; x++ {
			rgba := spr.Pixels[y*spr.Width+x]
			i := img.PixOffset(x, y)
			img.Pix[i] = byte(rgba >> 24)
			img.Pix[i+1] = byte(rgba >> 16)
			img.Pix[i+2] = byte(rgba >> 8)
			img.Pix[i+3] = byte(rgba)
		}
	}
	return img
}

func planeImage(data *wl6.MapData, plane, scale int) image.Image {
	w := data.Width()
	h := data.Height()
	img := image.NewRGBA(image.Rect(0, 0, w*scale, h*scale))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := valueColor(data.Cell(plane, x, y))
			fillRect(img, x*scale, y*scale, scale, scale, c)
		}
	}
	return img
}

func mapOverviewImage(level *wl6.Level, walls *wl6.WallSet) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, level.Width*mapOverviewTilePx, level.Height*mapOverviewTilePx))
	floor := color.RGBA{R: 64, G: 70, B: 60, A: 255}
	void := color.RGBA{R: 18, G: 20, B: 24, A: 255}
	for y := 0; y < level.Height; y++ {
		for x := 0; x < level.Width; x++ {
			tile := level.Tile(x, y)
			px := x * mapOverviewTilePx
			py := y * mapOverviewTilePx
			if !tile.RenderWall && !tile.Solid {
				fillRect(img, px, py, mapOverviewTilePx, mapOverviewTilePx, floor)
			} else if !tile.RenderWall {
				fillRect(img, px, py, mapOverviewTilePx, mapOverviewTilePx, void)
			}
			if tex, ok := mapTileTexture(tile, walls); ok {
				scaleNearest(img, image.Rect(px, py, px+mapOverviewTilePx, py+mapOverviewTilePx), tex)
			}
		}
	}
	for _, start := range level.PlayerStarts {
		fillRect(img, start.X*mapOverviewTilePx+mapOverviewTilePx/4, start.Y*mapOverviewTilePx+mapOverviewTilePx/4, mapOverviewTilePx/2, mapOverviewTilePx/2, color.RGBA{R: 240, G: 56, B: 36, A: 255})
	}
	return img
}

func mapTileTexture(tile wl6.Tile, walls *wl6.WallSet) (image.Image, bool) {
	if walls == nil {
		return nil, false
	}
	if tile.Door != nil {
		index := int(tile.RawWall)
		if index >= 0 && index < len(walls.Horizontal) && !walls.Horizontal[index].Empty() {
			return wallTextureImage(walls.Horizontal[index]), true
		}
		return nil, false
	}
	if !tile.RenderWall {
		return nil, false
	}
	index := int(tile.RawWall)
	if index >= 0 && index < len(walls.Horizontal) && !walls.Horizontal[index].Empty() {
		return wallTextureImage(walls.Horizontal[index]), true
	}
	return nil, false
}

func writePNG(path string, img image.Image, magnifyScale int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, magnifyImage(img, magnifyScale))
}

func sanitizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "unnamed"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if prevDash {
			continue
		}
		b.WriteByte('-')
		prevDash = true
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unnamed"
	}
	return out
}

func valueColor(v uint16) color.RGBA {
	if v == 0 {
		return color.RGBA{R: 16, G: 16, B: 16, A: 255}
	}
	return color.RGBA{
		R: uint8((v * 37) & 0xff),
		G: uint8((v * 73) & 0xff),
		B: uint8((v * 109) & 0xff),
		A: 255,
	}
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			img.SetRGBA(x+xx, y+yy, c)
		}
	}
}

func scaleNearest(dst *image.RGBA, rect image.Rectangle, src image.Image) {
	sb := src.Bounds()
	if rect.Empty() || sb.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		sy := sb.Min.Y + (y-rect.Min.Y)*sb.Dy()/rect.Dy()
		for x := rect.Min.X; x < rect.Max.X; x++ {
			sx := sb.Min.X + (x-rect.Min.X)*sb.Dx()/rect.Dx()
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func magnifyImage(src image.Image, scale int) image.Image {
	if scale <= 1 || src == nil {
		return src
	}
	sb := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, sb.Dx()*scale, sb.Dy()*scale))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		sy := sb.Min.Y + y/scale
		for x := 0; x < dst.Bounds().Dx(); x++ {
			sx := sb.Min.X + x/scale
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

type sheetEntry struct {
	Name  string
	Image image.Image
}

type sheetFrame struct {
	Name  string `json:"name"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	CellX int    `json:"cell_x"`
	CellY int    `json:"cell_y"`
	CellW int    `json:"cell_w"`
	CellH int    `json:"cell_h"`
	Index int    `json:"index"`
}

type sheetMetadata struct {
	Columns int          `json:"columns"`
	Rows    int          `json:"rows"`
	CellW   int          `json:"cell_w"`
	CellH   int          `json:"cell_h"`
	Frames  []sheetFrame `json:"frames"`
}

func writeSheetBundle(basePath string, entries []sheetEntry) error {
	if len(entries) == 0 {
		return nil
	}
	img, meta := buildSheet(entries)
	if err := writePNG(basePath+".png", img, 1); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sheet metadata: %w", err)
	}
	if err := os.WriteFile(basePath+".json", append(data, '\n'), 0o644); err != nil {
		return err
	}
	return nil
}

func buildSheet(entries []sheetEntry) (*image.RGBA, sheetMetadata) {
	cols := int(math.Ceil(math.Sqrt(float64(len(entries)))))
	if cols < 1 {
		cols = 1
	}
	rows := (len(entries) + cols - 1) / cols

	cellW := 1
	cellH := 1
	for _, entry := range entries {
		b := entry.Image.Bounds()
		if b.Dx() > cellW {
			cellW = b.Dx()
		}
		if b.Dy() > cellH {
			cellH = b.Dy()
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	meta := sheetMetadata{
		Columns: cols,
		Rows:    rows,
		CellW:   cellW,
		CellH:   cellH,
		Frames:  make([]sheetFrame, 0, len(entries)),
	}
	for i, entry := range entries {
		col := i % cols
		row := i / cols
		cellX := col * cellW
		cellY := row * cellH
		b := entry.Image.Bounds()
		x := cellX + (cellW-b.Dx())/2
		y := cellY + (cellH-b.Dy())/2
		drawImage(dst, x, y, entry.Image)
		meta.Frames = append(meta.Frames, sheetFrame{
			Name:  entry.Name,
			X:     x,
			Y:     y,
			W:     b.Dx(),
			H:     b.Dy(),
			CellX: cellX,
			CellY: cellY,
			CellW: cellW,
			CellH: cellH,
			Index: i,
		})
	}
	return dst, meta
}

func drawImage(dst *image.RGBA, x0, y0 int, src image.Image) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x0+x, y0+y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
}

func renderMusicChunkPCM(chunk *wl6.MusicChunk, sampleRate int) []byte {
	if chunk == nil || len(chunk.Events) == 0 || sampleRate <= 0 {
		return nil
	}
	tickFrames := sampleRate / musicTickRate
	if tickFrames < 1 {
		tickFrames = 1
	}
	synth := impsynth.New(sampleRate)
	synth.Reset()

	out := make([]byte, 0, len(chunk.Events)*tickFrames*4)
	for _, event := range chunk.Events {
		synth.WriteReg(event.Reg, event.Value)
		frames := int(event.Delay) * tickFrames
		if frames <= 0 {
			continue
		}
		pcm := synth.GenerateStereoS16(frames)
		start := len(out)
		out = append(out, make([]byte, len(pcm)*2)...)
		for i, sample := range pcm {
			binary.LittleEndian.PutUint16(out[start+i*2:], uint16(sample))
		}
	}
	return out
}

func wavBytes(sample []byte, sampleRate int) []byte {
	const (
		channels      = 2
		bitsPerSample = 16
	)
	blockAlign := channels * bitsPerSample / 8
	byteRate := sampleRate * blockAlign
	dataLen := len(sample)
	riffLen := 36 + dataLen
	buf := make([]byte, 44+dataLen)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(riffLen))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1)
	binary.LittleEndian.PutUint16(buf[22:], channels)
	binary.LittleEndian.PutUint32(buf[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:], bitsPerSample)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataLen))
	copy(buf[44:], sample)
	return buf
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
