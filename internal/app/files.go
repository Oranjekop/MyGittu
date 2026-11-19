package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"mygittu/internal/repository"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

func (s *Service) Files(ctx context.Context, id string) ([]string, error) {
	result, err := s.Read(ctx, id, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for path := range strings.SplitSeq(string(result.Stdout), "\x00") {
		if path != "" {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files, nil
}

type FilePreview struct {
	New        string `json:"new"`
	Binary     bool   `json:"binary"`
	NextOffset int64  `json:"next_offset,omitempty"`
	Version    string `json:"version,omitempty"`
	ImagePNG   []byte `json:"image_png,omitempty"`
}

const (
	maxPreviewTextBytes = 256 << 10
	maxPreviewTextLines = 8000
	maxImageInputBytes  = 128 << 20
	maxImagePixels      = 48_000_000
	maxThumbnailEdge    = 1024
)

// Decoding full-size images is the most memory-intensive part of preview.
var imagePreviewSlot = make(chan struct{}, 1)

func (s *Service) PreviewFile(ctx context.Context, id, path string, offset int64, version string) (FilePreview, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return FilePreview{}, err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return FilePreview{}, err
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.EqualFold(part, ".git") {
			return FilePreview{}, fmt.Errorf("Git metadata cannot be previewed")
		}
	}
	file, link, err := openWorktreeFile(repo.Root, path)
	if err != nil {
		return FilePreview{}, err
	}
	if file == nil {
		if offset != 0 || version != "" {
			return FilePreview{}, fmt.Errorf("file changed during preview; reopen it")
		}
		return FilePreview{New: link, Binary: link == ""}, nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return FilePreview{}, err
	}
	currentVersion := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	if version != "" && version != currentVersion {
		return FilePreview{}, fmt.Errorf("file changed during preview; reopen it")
	}
	if offset < 0 || offset > info.Size() || offset > 0 && version == "" {
		return FilePreview{}, fmt.Errorf("invalid file preview offset")
	}
	if version == "" {
		preview, recognized, err := detectImagePreview(ctx, file)
		if recognized || err != nil {
			return preview, err
		}
	}
	data, err := io.ReadAll(io.NewSectionReader(file, offset, maxPreviewTextBytes+1))
	if err != nil {
		return FilePreview{}, err
	}
	updated, err := file.Stat()
	if err != nil {
		return FilePreview{}, err
	}
	if updated.Size() != info.Size() || !updated.ModTime().Equal(info.ModTime()) {
		return FilePreview{}, fmt.Errorf("file changed during preview; reopen it")
	}
	return previewText(data, offset, currentVersion), nil
}

func detectImagePreview(ctx context.Context, file io.ReaderAt) (FilePreview, bool, error) {
	var header [4]byte
	n, _ := file.ReadAt(header[:], 0)
	isTIFF := n == len(header) && (string(header[:]) == "II*\x00" || string(header[:]) == "MM\x00*")
	input := io.NewSectionReader(file, 0, maxImageInputBytes)
	var config image.Config
	var imageErr error
	if isTIFF {
		config, imageErr = tiff.DecodeConfig(input)
	} else {
		config, _, imageErr = image.DecodeConfig(input)
	}
	if imageErr == nil {
		if _, err := input.Seek(0, io.SeekStart); err != nil {
			return FilePreview{}, true, err
		}
		preview, err := previewImage(ctx, input, config, isTIFF)
		return preview, true, err
	}
	return FilePreview{}, false, nil
}

func previewImage(ctx context.Context, input io.Reader, config image.Config, isTIFF bool) (FilePreview, error) {
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxImagePixels/config.Height {
		return FilePreview{}, fmt.Errorf("image is too large to preview (maximum 48 megapixels)")
	}
	select {
	case imagePreviewSlot <- struct{}{}:
		defer func() { <-imagePreviewSlot }()
	case <-ctx.Done():
		return FilePreview{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return FilePreview{}, err
	}
	var src image.Image
	var err error
	if isTIFF {
		src, err = tiff.Decode(input)
	} else {
		src, _, err = image.Decode(input)
	}
	if err != nil {
		return FilePreview{}, fmt.Errorf("could not decode image preview: %w", err)
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if longest := max(width, height); longest > maxThumbnailEdge {
		width = max(1, width*maxThumbnailEdge/longest)
		height = max(1, height*maxThumbnailEdge/longest)
	}
	// RGBA8 bounds the PNG and its base64 JSON envelope below the IPC frame limit.
	thumbnail := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(thumbnail, thumbnail.Bounds(), src, bounds, draw.Src, nil)
	var encoded bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&encoded, thumbnail); err != nil {
		return FilePreview{}, err
	}
	return FilePreview{ImagePNG: encoded.Bytes()}, nil
}

func previewText(data []byte, offset int64, version string) FilePreview {
	end := len(data)
	if end > maxPreviewTextBytes {
		end = maxPreviewTextBytes
		for end > maxPreviewTextBytes-utf8.UTFMax && !utf8.RuneStart(data[end]) {
			end--
		}
		if data[end-1] == '\r' && data[end] == '\n' {
			end--
		}
		if lineEnd := bytes.LastIndexByte(data[:end], '\n'); lineEnd >= 0 {
			end = lineEnd + 1
		}
	}
	page := data[:end]
	if bytes.IndexByte(page, 0) >= 0 || !utf8.Valid(page) {
		return FilePreview{Binary: true}
	}
	lineEnd := 0
	for line := 0; line < maxPreviewTextLines; line++ {
		next := bytes.IndexByte(page[lineEnd:], '\n')
		if next < 0 {
			break
		}
		lineEnd += next + 1
		if line == maxPreviewTextLines-1 {
			end = lineEnd
		}
	}
	result := FilePreview{New: strings.ReplaceAll(string(page[:end]), "\r\n", "\n"), Version: version}
	if end < len(data) {
		result.NextOffset = offset + int64(end)
	}
	return result
}
