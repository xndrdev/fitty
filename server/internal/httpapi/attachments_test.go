package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"testing"

	"fitty/server/internal/storage"
)

func attachmentPNG(t *testing.T, width, height int, fill color.Color) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(picture, picture.Bounds(), image.NewUniform(fill), image.Point{}, draw.Src)
	var data bytes.Buffer
	if err := png.Encode(&data, picture); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func attachmentJPEG(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	picture := image.NewGray(image.Rect(0, 0, 3, 2))
	if err := jpeg.Encode(&data, picture, nil); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestNormalizeSupportedImages(t *testing.T) {
	// A generated white 2x2 lossless WebP, independent of external test files.
	webp, err := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvAUAAAAfQ//73v/+BiOh/AAA=")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, mime    string
		data          []byte
		width, height int
	}{
		{"JPEG", "image/jpeg", attachmentJPEG(t), 3, 2},
		{"PNG", "image/png", attachmentPNG(t, 3, 2, color.White), 3, 2},
		{"WebP", "image/webp", webp, 2, 2},
		{"maximum width", "image/png", attachmentPNG(t, 4096, 1, color.White), 4096, 1},
		{"maximum height", "image/png", attachmentPNG(t, 1, 4096, color.White), 1, 4096},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, width, height, err := normalizeImage(test.data, test.mime)
			if err != nil {
				t.Fatal(err)
			}
			if width != test.width || height != test.height || http.DetectContentType(data) != "image/jpeg" {
				t.Fatalf("unexpected normalized image: %dx%d, %s", width, height, http.DetectContentType(data))
			}
			decoded, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil || decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
				t.Fatalf("normalization must produce a complete JPEG with the stated dimensions: %v", err)
			}
		})
	}
}

func TestNormalizeRejectsUnsupportedAndDamagedImages(t *testing.T) {
	pngData := attachmentPNG(t, 2, 2, color.White)
	jpegData := attachmentJPEG(t)
	tests := []struct {
		name, mime string
		data       []byte
	}{
		{"empty", "image/jpeg", nil},
		{"text", "image/jpeg", []byte("not an image")},
		{"SVG", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		{"GIF", "image/gif", []byte("GIF89a")},
		{"wrong declaration", "image/jpeg", pngData},
		{"missing declaration", "", pngData},
		{"unsupported declaration", "application/octet-stream", jpegData},
		{"truncated header", "image/png", pngData[:20]},
		{"truncated pixels", "image/png", pngData[:len(pngData)/2]},
		{"truncated JPEG", "image/jpeg", jpegData[:len(jpegData)-20]},
		{"oversize file", "image/png", append(append([]byte{}, pngData...), make([]byte, storage.MaxBytes+1-len(pngData))...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, width, height, err := normalizeImage(test.data, test.mime)
			if err == nil || data != nil || width != 0 || height != 0 {
				t.Fatalf("invalid upload produced an image: %dx%d, err=%v", width, height, err)
			}
		})
	}
}

func TestNormalizeRejectsDimensionsBeforeDecodingPixels(t *testing.T) {
	for _, size := range []struct{ width, height uint32 }{
		{0, 1}, {1, 0}, {4097, 1}, {1, 4097}, {4000, 3001}, {4096, 4096}, {0xffffffff, 0xffffffff},
	} {
		data := attachmentPNG(t, 1, 1, color.White)
		// Update IHDR and its checksum: these dimensions are valid PNG metadata,
		// but the upload must be refused before allocating its declared pixels.
		binary.BigEndian.PutUint32(data[16:20], size.width)
		binary.BigEndian.PutUint32(data[20:24], size.height)
		binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
		if result, _, _, err := normalizeImage(data, "image/png"); err == nil || result != nil {
			t.Errorf("accepted invalid dimensions %dx%d", size.width, size.height)
		}
	}
}

func TestNormalizeRemovesMetadataAndTrailingBytes(t *testing.T) {
	const metadata = "Exif\x00\x00private-location-test-marker"
	const trailing = "private-trailing-test-marker"
	jpegData := attachmentJPEG(t)
	segment := []byte{0xff, 0xe1, 0, byte(len(metadata) + 2)}
	segment = append(segment, metadata...)
	withMetadata := append(append(append([]byte{}, jpegData[:2]...), segment...), jpegData[2:]...)
	withMetadata = append(withMetadata, trailing...)
	result, _, _, err := normalizeImage(withMetadata, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(result, []byte(metadata)) || bytes.Contains(result, []byte(trailing)) || !bytes.HasSuffix(result, []byte{0xff, 0xd9}) {
		t.Fatal("normalized image retained metadata or trailing bytes")
	}

	// The exact byte limit remains valid; padding is discarded on re-encoding.
	pngData := attachmentPNG(t, 2, 2, color.White)
	padded := append(pngData, make([]byte, storage.MaxBytes-len(pngData))...)
	result, _, _, err = normalizeImage(padded, "image/png")
	if err != nil || len(result) >= 4096 {
		t.Fatalf("maximum-size input should normalize to the small actual image: length=%d err=%v", len(result), err)
	}
}

func TestNormalizeFlattensTransparencyOntoWhite(t *testing.T) {
	for _, test := range []struct {
		name string
		fill color.NRGBA
		want uint8
	}{
		{"transparent red", color.NRGBA{R: 255, A: 0}, 255},
		{"half-transparent black", color.NRGBA{A: 127}, 128},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, _, _, err := normalizeImage(attachmentPNG(t, 16, 16, test.fill), "image/png")
			if err != nil {
				t.Fatal(err)
			}
			picture, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			pixel := color.NRGBAModel.Convert(picture.At(8, 8)).(color.NRGBA)
			for _, channel := range []uint8{pixel.R, pixel.G, pixel.B} {
				difference := int(channel) - int(test.want)
				if difference < -2 || difference > 2 {
					t.Fatalf("expected neutral %d against white, got %#v", test.want, pixel)
				}
			}
			if pixel.A != 255 {
				t.Fatal("JPEG must be fully opaque")
			}
		})
	}
}
