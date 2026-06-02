package bot

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const (
	sampleRate = 48000
	channels   = 1
	frameSize  = 960 // 20ms at 48kHz
	frameDur   = 20 * time.Millisecond
)

// DecodeMP3ToPCM decodes an MP3 file to raw PCM (48kHz, mono, int16) using ffmpeg.
func DecodeMP3ToPCM(mp3Path string) ([]int16, error) {
	tmpFile, err := os.CreateTemp("", "audiobot-pcm-*.raw")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd := exec.Command("ffmpeg", "-y", "-i", mp3Path, "-f", "s16le", "-ar", "48000", "-ac", "1", tmpPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode: %w, output: %s", err, string(output))
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read PCM file: %w", err)
	}

	sampleCount := len(data) / 2
	pcm := make([]int16, sampleCount)
	for i := range pcm {
		pcm[i] = int16(data[2*i]) | int16(data[2*i+1])<<8
	}

	return pcm, nil
}

// EncodePCMToOpusFrames encodes PCM int16 data into Opus frames (20ms each) using ffmpeg.
func EncodePCMToOpusFrames(pcmData []int16) ([][]byte, error) {
	tmpPCM, err := os.CreateTemp("", "audiobot-pcm-in-*.raw")
	if err != nil {
		return nil, fmt.Errorf("create pcm temp: %w", err)
	}
	tmpPCMPath := tmpPCM.Name()

	buf := new(bytes.Buffer)
	for _, sample := range pcmData {
		if err := binary.Write(buf, binary.LittleEndian, sample); err != nil {
			os.Remove(tmpPCMPath)
			return nil, fmt.Errorf("write pcm sample: %w", err)
		}
	}
	if err := os.WriteFile(tmpPCMPath, buf.Bytes(), 0644); err != nil {
		os.Remove(tmpPCMPath)
		return nil, fmt.Errorf("write pcm file: %w", err)
	}
	defer os.Remove(tmpPCMPath)

	tmpOgg, err := os.CreateTemp("", "audiobot-opus-*.ogg")
	if err != nil {
		return nil, fmt.Errorf("create ogg temp: %w", err)
	}
	tmpOggPath := tmpOgg.Name()
	tmpOgg.Close()
	defer os.Remove(tmpOggPath)

	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "1",
		"-i", tmpPCMPath,
		"-c:a", "libopus",
		"-b:a", "24k",
		"-vbr", "off",
		"-frame_duration", "20",
		"-application", "voip",
		tmpOggPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg opus encode: %w, output: %s", err, string(output))
	}

	oggData, err := os.ReadFile(tmpOggPath)
	if err != nil {
		return nil, fmt.Errorf("read ogg file: %w", err)
	}

	frames, err := extractOpusFramesFromOgg(oggData)
	if err != nil {
		return nil, fmt.Errorf("extract opus frames: %w", err)
	}

	if len(frames) == 0 {
		return nil, fmt.Errorf("no opus frames produced")
	}

	return frames, nil
}

// extractOpusFramesFromOgg extracts raw Opus frames from an Ogg container.
func extractOpusFramesFromOgg(data []byte) ([][]byte, error) {
	var frames [][]byte
	i := 0

	for i < len(data) {
		if i+4 > len(data) || string(data[i:i+4]) != "OggS" {
			i++
			continue
		}

		if i+27 > len(data) {
			break
		}

		segmentCount := int(data[i+26])
		headerSize := 27 + segmentCount

		if i+headerSize > len(data) {
			break
		}

		dataSize := 0
		for j := 0; j < segmentCount; j++ {
			dataSize += int(data[i+27+j])
		}

		pageDataStart := i + headerSize
		pageDataEnd := pageDataStart + dataSize

		if pageDataEnd > len(data) {
			break
		}

		pageData := data[pageDataStart:pageDataEnd]
		if len(pageData) >= 8 && (string(pageData[:8]) == "OpusHead" || string(pageData[:8]) == "OpusTags") {
			i = pageDataEnd
			continue
		}

		offset := 0
		for j := 0; j < segmentCount; j++ {
			segSize := int(data[i+27+j])
			if segSize == 0 {
				continue
			}

			packetSize := segSize
			for j+1 < segmentCount && data[i+27+j] == 255 {
				j++
				packetSize += int(data[i+27+j])
			}

			packetStart := offset
			packetEnd := offset + packetSize

			if packetEnd <= len(pageData) && packetSize > 0 {
				frame := make([]byte, packetSize)
				copy(frame, pageData[packetStart:packetEnd])
				frames = append(frames, frame)
			}

			offset = packetEnd
		}

		i = pageDataEnd
	}

	return frames, nil
}

// LoadAudioFile loads an MP3 file and returns Opus frames ready for WebRTC.
func LoadAudioFile(mp3Path string) ([][]byte, error) {
	if filepath.Ext(mp3Path) == "" {
		mp3Path += ".mp3"
	}

	log.Printf("[audio] loading file: %s", mp3Path)

	fi, err := os.Stat(mp3Path)
	if err != nil {
		return nil, fmt.Errorf("stat mp3: %w", err)
	}
	log.Printf("[audio] file size: %d bytes", fi.Size())

	// Verify ffmpeg is available.
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	pcm, err := DecodeMP3ToPCM(mp3Path)
	if err != nil {
		return nil, fmt.Errorf("decode mp3: %w", err)
	}
	log.Printf("[audio] decoded PCM: %d samples", len(pcm))

	frames, err := EncodePCMToOpusFrames(pcm)
	if err != nil {
		return nil, fmt.Errorf("encode opus: %w", err)
	}

	log.Printf("[audio] opus frames: %d frames ready", len(frames))
	return frames, nil
}

// FrameIterator provides an iterator over Opus frames with optional looping.
type FrameIterator struct {
	frames [][]byte
	loop   bool
	index  int
}

// NewFrameIterator creates a new frame iterator.
func NewFrameIterator(frames [][]byte, loop bool) *FrameIterator {
	return &FrameIterator{frames: frames, loop: loop}
}

// Next returns the next Opus frame. Returns false if not looping and at the end.
func (fi *FrameIterator) Next() ([]byte, bool) {
	if fi.index >= len(fi.frames) {
		if fi.loop {
			fi.index = 0
		} else {
			return nil, false
		}
	}
	frame := fi.frames[fi.index]
	fi.index++
	return frame, true
}

// StreamOpusFrames writes Opus frames to a track at 20ms intervals.
// It blocks until context is cancelled or frames are exhausted (non-looping).
func StreamOpusFrames(ctx context.Context, track *webrtc.TrackLocalStaticSample, frames [][]byte, loop bool) error {
	iter := NewFrameIterator(frames, loop)
	ticker := time.NewTicker(frameDur)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			frame, ok := iter.Next()
			if !ok {
				return nil
			}

			sample := media.Sample{
				Data:     frame,
				Duration: frameDur,
			}
			if err := track.WriteSample(sample); err != nil {
				return fmt.Errorf("write sample: %w", err)
			}
		}
	}
}
