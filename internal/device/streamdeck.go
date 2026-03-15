package device

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"sync"

	"github.com/sstallion/go-hid"
	"golang.org/x/image/draw"
)

const VendorID = 0x0fd9

// ModelInfo describes hardware-specific constants for a Stream Deck model.
type ModelInfo struct {
	KeyCount    int
	Cols        int
	Rows        int
	ImageWidth  int
	ImageHeight int
	// FlipX/FlipY: the XL renders images mirrored; we pre-flip before sending.
	FlipX bool
	FlipY bool
}

// models maps USB product IDs to their hardware specs.
var models = map[uint16]ModelInfo{
	0x00ba: {KeyCount: 32, Cols: 8, Rows: 4, ImageWidth: 96, ImageHeight: 96, FlipX: true, FlipY: true}, // XL v2
	0x006c: {KeyCount: 32, Cols: 8, Rows: 4, ImageWidth: 96, ImageHeight: 96, FlipX: true, FlipY: true}, // XL v1
	0x006d: {KeyCount: 15, Cols: 5, Rows: 3, ImageWidth: 72, ImageHeight: 72, FlipX: true, FlipY: true}, // MK.2
}

const (
	reportSize       = 1024
	imageHeaderSize  = 8
	imagePayloadSize = reportSize - imageHeaderSize
	readReportSize   = 512
)

// StreamDeck represents an open Stream Deck device.
type StreamDeck struct {
	mu    sync.Mutex
	dev   *hid.Device
	model ModelInfo
}

// Open finds and opens the first Stream Deck with the given product ID.
func Open(vendorID, productID uint16) (*StreamDeck, error) {
	if err := hid.Init(); err != nil {
		return nil, fmt.Errorf("hid init: %w", err)
	}

	m, ok := models[productID]
	if !ok {
		return nil, fmt.Errorf("unsupported product ID: 0x%04x (add it to internal/device/streamdeck.go)", productID)
	}

	dev, err := hid.OpenFirst(vendorID, productID)
	if err != nil {
		return nil, fmt.Errorf("open device 0x%04x:0x%04x: %w (try: sudo chmod a+rw /dev/hidraw*)", vendorID, productID, err)
	}

	return &StreamDeck{dev: dev, model: m}, nil
}

// Close releases the device.
func (sd *StreamDeck) Close() error {
	_ = hid.Exit()
	return sd.dev.Close()
}

// KeyCount returns the number of keys on this device.
func (sd *StreamDeck) KeyCount() int { return sd.model.KeyCount }

// Reset clears all key images and returns the device to its default state.
func (sd *StreamDeck) Reset() error {
	report := make([]byte, 32)
	report[0] = 0x03
	report[1] = 0x02
	_, err := sd.dev.SendFeatureReport(report)
	return err
}

// SetBrightness sets the display brightness (0–100).
func (sd *StreamDeck) SetBrightness(pct int) error {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	report := make([]byte, 32)
	report[0] = 0x03
	report[1] = 0x08
	report[2] = byte(pct)
	_, err := sd.dev.SendFeatureReport(report)
	return err
}

// EncodeFrame scales img to key size and returns the JPEG bytes ready to send.
// Use this to pre-encode animation frames once rather than on every tick.
func (sd *StreamDeck) EncodeFrame(img image.Image) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, sd.model.ImageWidth, sd.model.ImageHeight))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	if sd.model.FlipX || sd.model.FlipY {
		dst = flipImage(dst, sd.model.FlipX, sd.model.FlipY)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SetKeyFrame sends pre-encoded JPEG bytes (from EncodeFrame) to a key.
func (sd *StreamDeck) SetKeyFrame(keyIndex int, jpegData []byte) error {
	return sd.sendKeyImageData(keyIndex, jpegData)
}

// SetKeyImage scales img to the key size and sends it to the given key (0-indexed).
func (sd *StreamDeck) SetKeyImage(keyIndex int, img image.Image) error {
	// Scale to key pixel dimensions.
	dst := image.NewRGBA(image.Rect(0, 0, sd.model.ImageWidth, sd.model.ImageHeight))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	// The XL renders images flipped; pre-flip so they appear correct.
	if sd.model.FlipX || sd.model.FlipY {
		dst = flipImage(dst, sd.model.FlipX, sd.model.FlipY)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 95}); err != nil {
		return fmt.Errorf("jpeg encode: %w", err)
	}

	return sd.sendKeyImageData(keyIndex, buf.Bytes())
}

// ClearKey fills the given key with solid black.
func (sd *StreamDeck) ClearKey(keyIndex int) error {
	blank := image.NewRGBA(image.Rect(0, 0, sd.model.ImageWidth, sd.model.ImageHeight))
	return sd.SetKeyImage(keyIndex, blank)
}

// ReadButtons waits up to 250 ms for a button state report.
// Returns (nil, nil) on timeout — callers should check context and retry.
func (sd *StreamDeck) ReadButtons() ([]bool, error) {
	data := make([]byte, readReportSize)
	n, err := sd.dev.ReadWithTimeout(data, 250)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil // timeout, no data
	}

	// Input report layout (XL v2): [report_id, 0x00, 0x00, 0x00, key0, key1, ...]
	const offset = 4
	if n < offset+sd.model.KeyCount {
		return nil, fmt.Errorf("short read: got %d bytes, expected at least %d", n, offset+sd.model.KeyCount)
	}

	buttons := make([]bool, sd.model.KeyCount)
	for i := 0; i < sd.model.KeyCount; i++ {
		buttons[i] = data[offset+i] == 0x01
	}
	return buttons, nil
}

// sendKeyImageData sends raw JPEG bytes to a key, split across 1024-byte HID reports.
func (sd *StreamDeck) sendKeyImageData(keyIndex int, data []byte) error {
	if keyIndex < 0 || keyIndex >= sd.model.KeyCount {
		return fmt.Errorf("key index %d out of range (device has %d keys, 0–%d)",
			keyIndex, sd.model.KeyCount, sd.model.KeyCount-1)
	}
	sd.mu.Lock()
	defer sd.mu.Unlock()
	pageIndex := 0
	for len(data) > 0 {
		chunk := data
		if len(chunk) > imagePayloadSize {
			chunk = chunk[:imagePayloadSize]
		}

		isLast := byte(0)
		if len(data) == len(chunk) {
			isLast = 1
		}

		report := make([]byte, reportSize)
		report[0] = 0x02
		report[1] = 0x07
		report[2] = byte(keyIndex)
		report[3] = isLast
		binary.LittleEndian.PutUint16(report[4:6], uint16(len(chunk)))
		binary.LittleEndian.PutUint16(report[6:8], uint16(pageIndex))
		copy(report[8:], chunk)

		if _, err := sd.dev.Write(report); err != nil {
			return fmt.Errorf("write report page %d: %w", pageIndex, err)
		}

		data = data[len(chunk):]
		pageIndex++
	}
	return nil
}

func flipImage(src *image.RGBA, flipX, flipY bool) *image.RGBA {
	b := src.Bounds()
	w, h := b.Max.X, b.Max.Y
	dst := image.NewRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			if flipX {
				dx = w - 1 - x
			}
			if flipY {
				dy = h - 1 - y
			}
			dst.SetRGBA(dx, dy, src.RGBAAt(x, y))
		}
	}
	return dst
}
