// SPDX-License-Identifier: MIT

package rfstream

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func TestEncodeContextMatchesCRTEmulatorDIFIProfile(t *testing.T) {
	stamp := timestamp{seconds: 1_700_000_000, picoseconds: 123_000_000_000}
	packet := EncodeContextPacket(0x4e545343, 7, stamp, 189_000_000, true, false)
	if len(packet) != ContextPacketBytes {
		t.Fatalf("context bytes=%d, want %d", len(packet), ContextPacketBytes)
	}
	assertHeader(t, packet, 4, 1, 7, 27)
	if got := binary.BigEndian.Uint32(packet[4:8]); got != 0x4e545343 {
		t.Fatalf("stream ID=%#x", got)
	}
	if got := binary.BigEndian.Uint32(packet[8:12]); got != DIFIOUI {
		t.Fatalf("OUI=%#x", got)
	}
	if got := binary.BigEndian.Uint16(packet[14:16]); got != 1 {
		t.Fatalf("packet class=%d", got)
	}
	if got := binary.BigEndian.Uint32(packet[28:32]); got != 0xfbb98000 {
		t.Fatalf("CIF0=%#x", got)
	}
	if got := binary.BigEndian.Uint32(packet[32:36]); got != 75 {
		t.Fatalf("reference point=%d", got)
	}
	if got := binary.BigEndian.Uint64(packet[36:44]); got != uint64(6_000_000)<<20 {
		t.Fatalf("bandwidth Q44.20=%#x", got)
	}
	if got := int64(binary.BigEndian.Uint64(packet[52:60])); got != int64(189_000_000)<<20 {
		t.Fatalf("RF center Q44.20=%#x", got)
	}
	if got := binary.BigEndian.Uint64(packet[76:84]); got != DIFIProfileSampleRateRaw {
		t.Fatalf("sample rate raw=%#x", got)
	}
	state := binary.BigEndian.Uint32(packet[96:100])
	if state&(1<<18) == 0 || state&(1<<15) == 0 || state&(1<<14) != 0 {
		t.Fatalf("state word=%#x", state)
	}
	if got := binary.BigEndian.Uint64(packet[100:108]); got != DIFIProfileDataFormat {
		t.Fatalf("data format=%#x", got)
	}
}

func TestEncodeContextSetsSampleLossIndicator(t *testing.T) {
	packet := EncodeContextPacket(1, 0, timestamp{}, DefaultRFCenterHz, false, true)
	state := binary.BigEndian.Uint32(packet[96:100])
	if state&(1<<12) == 0 {
		t.Fatalf("state word=%#x, sample-loss indicator is clear", state)
	}
}

func TestEncodeDataUsesBigEndianQ115AndRationalTimestamp(t *testing.T) {
	iq := []IQSample{{I: -32768, Q: 32767}, {I: 0x1234, Q: -0x1234}}
	stamp := timestampAtSample(timestamp{seconds: 1_700_000_000, picoseconds: 100}, 356)
	packet, err := EncodeDataPacket(0x4e545343, 15, stamp, iq)
	if err != nil {
		t.Fatal(err)
	}
	assertHeader(t, packet, 1, 0, 15, uint16(len(packet)/4))
	wantPayload := []byte{0x80, 0x00, 0x7f, 0xff, 0x12, 0x34, 0xed, 0xcc}
	if got := packet[DataHeaderBytes:]; string(got) != string(wantPayload) {
		t.Fatalf("payload=%x, want %x", got, wantPayload)
	}
	wantPs := uint64(100 + roundedSamplePicoseconds(356))
	if got := binary.BigEndian.Uint64(packet[20:28]); got != wantPs {
		t.Fatalf("fractional timestamp=%d, want %d", got, wantPs)
	}
}

func TestEncodeDataPacketIntoReusesCallerBuffer(t *testing.T) {
	buffer := make([]byte, DataHeaderBytes+8, DataHeaderBytes+64)
	packet, err := EncodeDataPacketInto(buffer, 1, 2, timestamp{}, []IQSample{{I: 3, Q: 4}, {I: 5, Q: 6}})
	if err != nil {
		t.Fatal(err)
	}
	if &packet[0] != &buffer[0] {
		t.Fatal("data encoder did not reuse the caller buffer")
	}
}

func TestSynthesizeFrameIntoReusesCallerBuffer(t *testing.T) {
	frame := solidFrame(0, 0, 0)
	audio := make([]int16, StereoSamplesPerInputFrame)
	buffer := make([]IQSample, SamplesPerFrame)
	iq, err := NewSynthesizer().SynthesizeFrameInto(buffer, frame, frame, audio, audio)
	if err != nil {
		t.Fatal(err)
	}
	if &iq[0] != &buffer[0] {
		t.Fatal("synthesizer did not reuse the caller buffer")
	}
}

func TestEncodeVersionMatchesCRTEmulatorDecoder(t *testing.T) {
	packet := EncodeVersionPacket(
		0x4e545343,
		3,
		timestamp{seconds: 1_700_000_000, picoseconds: 0},
		time.Date(2026, time.July, 18, 0, 0, 0, 0, time.UTC),
		true,
	)
	if len(packet) != VersionPacketBytes {
		t.Fatalf("version bytes=%d", len(packet))
	}
	assertHeader(t, packet, 4, 1, 3, 11)
	if got := binary.BigEndian.Uint16(packet[12:14]); got != 1 {
		t.Fatalf("information class=%d", got)
	}
	if got := binary.BigEndian.Uint16(packet[14:16]); got != 4 {
		t.Fatalf("packet class=%d", got)
	}
	if got := binary.BigEndian.Uint32(packet[28:32]); got != 0x80000002 {
		t.Fatalf("CIF0=%#x", got)
	}
	if got := binary.BigEndian.Uint32(packet[32:36]); got != 0x0c {
		t.Fatalf("CIF1=%#x", got)
	}
	if got := binary.BigEndian.Uint32(packet[36:40]); got != 4 {
		t.Fatalf("VITA version=%d", got)
	}
}

func TestSynthesizeFrameContainsVideoAndFMAuralCarriers(t *testing.T) {
	first := solidFrame(0xff, 0xff, 0xff)
	second := solidFrame(0x00, 0x00, 0x00)
	audio := sineStereo(1_000, StereoSamplesPerInputFrame/2)
	synth := NewSynthesizer()
	iq, err := synth.SynthesizeFrame(first, second, audio, audio)
	if err != nil {
		t.Fatal(err)
	}
	if len(iq) != SamplesPerFrame {
		t.Fatalf("samples=%d, want %d", len(iq), SamplesPerFrame)
	}
	videoPower := tonePower(iq, -1_750_000)
	audioPower := tonePower(iq, 2_750_000)
	if videoPower < 1e12 {
		t.Fatalf("video carrier power=%g", videoPower)
	}
	if audioPower < 1e9 {
		t.Fatalf("aural carrier power=%g", audioPower)
	}
	activeWhite := magnitude(iq[VideoActiveLineStart*SamplesPerLine+VideoActiveSampleStart+300])
	activeBlack := magnitude(iq[SamplesPerField+VideoActiveLineStart*SamplesPerLine+VideoActiveSampleStart+300])
	if activeWhite >= activeBlack {
		t.Fatalf("negative modulation missing: white=%g black=%g", activeWhite, activeBlack)
	}
	for index, sample := range iq {
		if sample.I == math.MinInt16 || sample.I == math.MaxInt16 || sample.Q == math.MinInt16 || sample.Q == math.MaxInt16 {
			t.Fatalf("Q1.15 clipping at sample %d: %+v", index, sample)
		}
	}
}

func TestVSBFilterRejectsLowerSidebandAndKeepsUpperChannel(t *testing.T) {
	filter := designVSBFilter(vsbFilterTaps)
	blocked := filterPower(filter, -5_000_000)
	passed := filterPower(filter, 2_000_000)
	if passed < 0.25 {
		t.Fatalf("VSB passband power=%g", passed)
	}
	if blocked > passed/100 {
		t.Fatalf("lower sideband rejection=%g, passband=%g", blocked, passed)
	}
}

func TestQuantizeQ115SaturatesPositiveFullScale(t *testing.T) {
	for _, value := range []float64{1, math.Nextafter(1, 0), 0.99999, 2} {
		if got := quantizeQ115(float32(value)); got < 0 {
			t.Fatalf("quantizeQ115(%g)=%d, want positive saturation", value, got)
		}
	}
	if got := quantizeQ115(1); got != math.MaxInt16 {
		t.Fatalf("quantizeQ115(1)=%d, want %d", got, math.MaxInt16)
	}
}

func TestNTSCRasterHasSequentialFieldsAndSecondVerticalSync(t *testing.T) {
	first := solidFrame(0xff, 0xff, 0xff)
	second := solidFrame(0, 0, 0)
	activeColumn := VideoActiveSampleStart + 100
	firstFieldSample := 22*SamplesPerLine + activeColumn
	secondFieldSample := SamplesPerField + 22*SamplesPerLine + activeColumn
	if firstLevel := videoAmplitudeAt(first, second, firstFieldSample, uint64(firstFieldSample)); firstLevel >= 0.4 {
		t.Fatalf("first-field white level=%g, want low carrier envelope", firstLevel)
	}
	if secondLevel := videoAmplitudeAt(first, second, secondFieldSample, uint64(secondFieldSample)); secondLevel <= 0.6 {
		t.Fatalf("second-field black level=%g, want high carrier envelope", secondLevel)
	}
	verticalSyncSample := SamplesPerField + 6*SamplesPerHalfLine + 100
	if level := videoAmplitudeAt(first, second, verticalSyncSample, uint64(verticalSyncSample)); level != 1 {
		t.Fatalf("second-field vertical sync level=%g, want 1", level)
	}
	secondFieldHorizontalSync := SamplesPerField + 22*SamplesPerLine + 10
	if secondFieldHorizontalSync%SamplesPerLine != SamplesPerHalfLine+10 {
		t.Fatal("test did not exercise the half-line field offset")
	}
	if level := videoAmplitudeAt(first, second, secondFieldHorizontalSync, uint64(secondFieldHorizontalSync)); level != 1 {
		t.Fatalf("second-field horizontal sync level=%g, want 1", level)
	}
}

func TestExplicitNTSCInputRateConversionCadence(t *testing.T) {
	if got := sourceFieldIndex(999); got != 999 {
		t.Fatalf("source field 999=%d", got)
	}
	if got := sourceFieldIndex(1000); got != 1001 {
		t.Fatalf("source field 1000=%d, want controlled video drop", got)
	}
	var audioSamples int
	for frame := uint64(0); frame < 5; frame++ {
		audioSamples += audioSamplesForRFFrame(frame)
	}
	if audioSamples != 8008 {
		t.Fatalf("five RF frames consume %d audio samples, want 8008", audioSamples)
	}
	previous := sourceFieldIndex(0)
	dropped := uint64(0)
	for field := uint64(1); field <= 1000; field++ {
		current := sourceFieldIndex(field)
		if current <= previous {
			t.Fatalf("source field reversed at output %d: %d after %d", field, current, previous)
		}
		dropped += current - previous - 1
		previous = current
	}
	if dropped != 1 {
		t.Fatalf("1001 output fields dropped %d inputs, want exactly 1", dropped)
	}
	consumed := 0
	for frame := uint64(0); frame < 20; frame++ {
		start := float64(consumed) + float64((frame*8008)%5)/5
		want := float64(frame*8008) / 5
		if math.Abs(start-want) > 1e-9 {
			t.Fatalf("audio interpolation discontinuity at frame %d: %g, want %g", frame, start, want)
		}
		consumed += audioSamplesForRFFrame(frame)
	}
}

func BenchmarkSynthesizeFrame(b *testing.B) {
	first := solidFrame(0x20, 0x60, 0xa0)
	second := solidFrame(0xa0, 0x60, 0x20)
	audio := sineStereo(1_000, StereoSamplesPerInputFrame/2)
	synth := NewSynthesizer()
	b.ResetTimer()
	for range b.N {
		if _, err := synth.SynthesizeFrame(first, second, audio, audio); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSynthesizeFrameInto(b *testing.B) {
	first := solidFrame(0x20, 0x60, 0xa0)
	second := solidFrame(0xa0, 0x60, 0x20)
	audio := sineStereo(1_000, StereoSamplesPerInputFrame/2)
	synth := NewSynthesizer()
	buffer := make([]IQSample, SamplesPerFrame)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := synth.SynthesizeFrameInto(buffer, first, second, audio, audio); err != nil {
			b.Fatal(err)
		}
	}
}

func assertHeader(t *testing.T, packet []byte, packetType, tsm, sequence uint32, words uint16) {
	t.Helper()
	word := binary.BigEndian.Uint32(packet[:4])
	if got := word >> 28; got != packetType {
		t.Fatalf("packet type=%d, want %d", got, packetType)
	}
	if word&(1<<27) == 0 {
		t.Fatal("Class ID is absent")
	}
	if got := (word >> 24) & 1; got != tsm {
		t.Fatalf("TSM=%d, want %d", got, tsm)
	}
	if got := (word >> 22) & 3; got != 3 {
		t.Fatalf("TSI=%d", got)
	}
	if got := (word >> 20) & 3; got != 2 {
		t.Fatalf("TSF=%d", got)
	}
	if got := (word >> 16) & 0xf; got != sequence {
		t.Fatalf("sequence=%d, want %d", got, sequence)
	}
	if got := uint16(word); got != words {
		t.Fatalf("packet words=%d, want %d", got, words)
	}
}

func solidFrame(r, g, b byte) []byte {
	frame := make([]byte, RGBWidth*RGBHeight*3)
	for offset := 0; offset < len(frame); offset += 3 {
		frame[offset], frame[offset+1], frame[offset+2] = r, g, b
	}
	return frame
}

func sineStereo(frequency float64, frames int) []int16 {
	audio := make([]int16, frames*2)
	for index := 0; index < frames; index++ {
		value := int16(math.Round(math.Sin(2*math.Pi*frequency*float64(index)/InputAudioSampleRate) * 16_000))
		audio[index*2], audio[index*2+1] = value, value
	}
	return audio
}

func tonePower(iq []IQSample, frequency float64) float64 {
	var realPart, imagPart float64
	for index, sample := range iq {
		phase := -2 * math.Pi * frequency * float64(index) / SampleRate
		c, s := math.Cos(phase), math.Sin(phase)
		realPart += float64(sample.I)*c - float64(sample.Q)*s
		imagPart += float64(sample.I)*s + float64(sample.Q)*c
	}
	return realPart*realPart + imagPart*imagPart
}

func magnitude(sample IQSample) float64 {
	return math.Hypot(float64(sample.I), float64(sample.Q))
}

func filterPower(filter []carrierSample, frequency float64) float64 {
	var realPart, imagPart float64
	for index, coefficient := range filter {
		phase := -2 * math.Pi * frequency * float64(index) / SampleRate
		c, s := math.Cos(phase), math.Sin(phase)
		realPart += float64(coefficient.i)*c - float64(coefficient.q)*s
		imagPart += float64(coefficient.i)*s + float64(coefficient.q)*c
	}
	return realPart*realPart + imagPart*imagPart
}
