// SPDX-License-Identifier: MIT

package rfstream

import (
	"errors"
	"fmt"
	"math"
)

const (
	RGBWidth                   = 256
	RGBHeight                  = 240
	InputAudioSampleRate       = 48_000
	StereoSamplesPerInputFrame = InputAudioSampleRate / 60 * 2
	VideoActiveSampleStart     = 142
	VideoActiveLineStart       = 22
	VideoActiveLines           = 480
	videoCarrierHz             = -1_750_000.0
	auralCarrierHz             = 2_750_000.0
	auralDeviationHz           = 25_000.0
	videoScale                 = 0.82
	auralScale                 = 0.12
	phaseTableBits             = 16
	phaseTableSize             = 1 << phaseTableBits
	vsbFilterTaps              = 9
)

var ErrInvalidStereoAudio = errors.New("audio must be one exact interleaved stereo input frame")

var (
	chromaCos = [4]float64{1, 0, -1, 0}
	chromaSin = [4]float64{0, 1, 0, -1}
)

type carrierSample struct {
	i float32
	q float32
}

type Synthesizer struct {
	videoCarrier  [90]carrierSample
	phaseTable    [phaseTableSize]carrierSample
	vsbFilter     []carrierSample
	videoHistory  []carrierSample
	monoAudio     []float64
	rawVideo      []carrierSample
	filteredVideo []carrierSample
	initialized   bool
	auralPhase    uint32
	sampleIndex   uint64
}

func NewSynthesizer() *Synthesizer {
	synth := &Synthesizer{}
	for index := range synth.videoCarrier {
		phase := 2 * math.Pi * videoCarrierHz * float64(index) / SampleRate
		synth.videoCarrier[index] = carrierSample{i: float32(math.Cos(phase)), q: float32(math.Sin(phase))}
	}
	for index := range synth.phaseTable {
		phase := 2 * math.Pi * float64(index) / phaseTableSize
		synth.phaseTable[index] = carrierSample{i: float32(math.Cos(phase)), q: float32(math.Sin(phase))}
	}
	synth.vsbFilter = designVSBFilter(vsbFilterTaps)
	synth.videoHistory = make([]carrierSample, vsbFilterTaps-1)
	synth.monoAudio = make([]float64, StereoSamplesPerInputFrame+4)
	synth.rawVideo = make([]carrierSample, SamplesPerFrame)
	synth.filteredVideo = make([]carrierSample, SamplesPerFrame)
	return synth
}

func (s *Synthesizer) SynthesizeFrame(firstRGB, secondRGB []byte, firstAudio, secondAudio []int16) ([]IQSample, error) {
	return s.SynthesizeFrameInto(nil, firstRGB, secondRGB, firstAudio, secondAudio)
}

func (s *Synthesizer) SynthesizeFrameInto(destination []IQSample, firstRGB, secondRGB []byte, firstAudio, secondAudio []int16) ([]IQSample, error) {
	if len(firstRGB) != RGBWidth*RGBHeight*3 || len(secondRGB) != RGBWidth*RGBHeight*3 {
		return nil, fmt.Errorf("RGB24 inputs must each contain %d bytes", RGBWidth*RGBHeight*3)
	}
	if len(firstAudio) != StereoSamplesPerInputFrame || len(secondAudio) != StereoSamplesPerInputFrame {
		return nil, ErrInvalidStereoAudio
	}
	mono := downmixPairInto(s.monoAudio, firstAudio, secondAudio)
	last := mono[len(mono)-1]
	mono = s.monoAudio[:len(mono)+3]
	mono[len(mono)-3], mono[len(mono)-2], mono[len(mono)-1] = last, last, last
	return s.synthesizeFrameMonoInto(destination, firstRGB, secondRGB, mono, 0)
}

func (s *Synthesizer) synthesizeFrameMonoInto(destination []IQSample, firstRGB, secondRGB []byte, mono []float64, audioStart float64) ([]IQSample, error) {
	if len(firstRGB) != RGBWidth*RGBHeight*3 || len(secondRGB) != RGBWidth*RGBHeight*3 {
		return nil, fmt.Errorf("RGB24 inputs must each contain %d bytes", RGBWidth*RGBHeight*3)
	}
	maxAudioPosition := audioStart + float64(SamplesPerFrame-1)*InputAudioSampleRate/SampleRate
	if audioStart < 0 || len(mono) < int(maxAudioPosition)+2 {
		return nil, ErrInvalidStereoAudio
	}
	rawVideo := s.rawVideo
	baseAuralIncrement := phaseIncrement(auralCarrierHz)
	deviationIncrement := float64(uint64(1)<<32) * auralDeviationHz / SampleRate

	for line := 0; line < SamplesPerFrame/SamplesPerLine; line++ {
		for column := 0; column < SamplesPerLine; column++ {
			localIndex := line*SamplesPerLine + column
			globalIndex := s.sampleIndex + uint64(localIndex)
			waveIndex := (localIndex + len(s.vsbFilter)/2) % SamplesPerFrame
			amplitude := videoAmplitudeAt(firstRGB, secondRGB, waveIndex, globalIndex+uint64(len(s.vsbFilter)/2))
			video := s.videoCarrier[globalIndex%uint64(len(s.videoCarrier))]
			rawVideo[localIndex] = carrierSample{i: float32(amplitude) * video.i, q: float32(amplitude) * video.q}

		}
	}
	filteredVideo := s.filterVideo(rawVideo)
	if cap(destination) < SamplesPerFrame {
		destination = make([]IQSample, SamplesPerFrame)
	} else {
		destination = destination[:SamplesPerFrame]
	}
	iq := destination
	for index := range iq {
		audioPosition := audioStart + float64(index)*InputAudioSampleRate/SampleRate
		audioValue := interpolate(mono, audioPosition)
		increment := int64(baseAuralIncrement) + roundInt64(audioValue*deviationIncrement)
		s.auralPhase += uint32(increment)
		aural := s.phaseTable[s.auralPhase>>(32-phaseTableBits)]
		i := videoScale*filteredVideo[index].i + auralScale*aural.i
		q := videoScale*filteredVideo[index].q + auralScale*aural.q
		iq[index] = IQSample{I: quantizeQ115(i), Q: quantizeQ115(q)}
	}
	s.sampleIndex += SamplesPerFrame
	return iq, nil
}

func (s *Synthesizer) filterVideo(input []carrierSample) []carrierSample {
	if !s.initialized {
		copy(s.videoHistory, input[len(input)-len(s.videoHistory):])
		s.initialized = true
	}
	output := s.filteredVideo[:len(input)]
	historySamples := min(len(output), len(s.videoHistory))
	for outputIndex := 0; outputIndex < historySamples; outputIndex++ {
		var sumI, sumQ float32
		for tapIndex, coefficient := range s.vsbFilter {
			sourceIndex := outputIndex - tapIndex
			var sample carrierSample
			if sourceIndex >= 0 {
				sample = input[sourceIndex]
			} else {
				sample = s.videoHistory[len(s.videoHistory)+sourceIndex]
			}
			sumI += sample.i*coefficient.i - sample.q*coefficient.q
			sumQ += sample.i*coefficient.q + sample.q*coefficient.i
		}
		output[outputIndex] = carrierSample{i: sumI, q: sumQ}
	}
	coefficients := s.vsbFilter
	for outputIndex := historySamples; outputIndex < len(output); outputIndex++ {
		var sumI, sumQ float32
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex], coefficients[0])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-1], coefficients[1])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-2], coefficients[2])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-3], coefficients[3])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-4], coefficients[4])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-5], coefficients[5])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-6], coefficients[6])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-7], coefficients[7])
		sumI, sumQ = complexMultiplyAdd(sumI, sumQ, input[outputIndex-8], coefficients[8])
		output[outputIndex] = carrierSample{i: sumI, q: sumQ}
	}
	copy(s.videoHistory, input[len(input)-len(s.videoHistory):])
	return output
}

func complexMultiplyAdd(sumI, sumQ float32, sample, coefficient carrierSample) (float32, float32) {
	return sumI + sample.i*coefficient.i - sample.q*coefficient.q,
		sumQ + sample.i*coefficient.q + sample.q*coefficient.i
}

func designVSBFilter(taps int) []carrierSample {
	if taps < 3 || taps%2 == 0 {
		panic("VSB filter requires an odd tap count of at least three")
	}
	const lowHz = -2_500_000.0
	const highHz = 2_450_000.0
	center := (taps - 1) / 2
	filter := make([]carrierSample, taps)
	for index := range filter {
		k := float64(index - center)
		window := 0.54 - 0.46*math.Cos(2*math.Pi*float64(index)/float64(taps-1))
		if k == 0 {
			filter[index] = carrierSample{i: float32((highHz - lowHz) / SampleRate * window)}
			continue
		}
		highPhase := 2 * math.Pi * highHz * k / SampleRate
		lowPhase := 2 * math.Pi * lowHz * k / SampleRate
		denominator := 2 * math.Pi * k
		filter[index] = carrierSample{
			i: float32((math.Sin(highPhase) - math.Sin(lowPhase)) / denominator * window),
			q: float32((math.Cos(lowPhase) - math.Cos(highPhase)) / denominator * window),
		}
	}
	return filter
}

func videoAmplitudeAt(firstFrame, secondFrame []byte, frameSample int, globalIndex uint64) float64 {
	fieldSample := frameSample
	frame := firstFrame
	if frameSample >= SamplesPerField {
		fieldSample -= SamplesPerField
		frame = secondFrame
	}
	halfLine := fieldSample / SamplesPerHalfLine
	halfColumn := fieldSample % SamplesPerHalfLine
	if halfLine < 6 || (halfLine >= 12 && halfLine < 18) {
		// Equalizing pulses are half-line-rate narrow sync pulses.
		if halfColumn < 34 {
			return 1
		}
		return 0.75
	}
	if halfLine < 12 {
		// Vertical-sync serrations leave a short blanking gap each half-line.
		if halfColumn < 390 {
			return 1
		}
		return 0.75
	}
	line := fieldSample / SamplesPerLine
	column := fieldSample % SamplesPerLine
	if column < 68 {
		return 1
	}
	activeLine := line - VideoActiveLineStart
	if activeLine < 0 || activeLine >= RGBHeight {
		return 0.75
	}
	if column >= 76 && column < 112 {
		return clamp(0.75+0.08*chromaSin[globalIndex&3], 0.05, 1)
	}
	if column < VideoActiveSampleStart {
		return 0.75
	}

	activeColumn := column - VideoActiveSampleStart
	x := activeColumn * RGBWidth / (SamplesPerLine - VideoActiveSampleStart)
	y := activeLine
	offset := (y*RGBWidth + x) * 3
	r := float64(frame[offset]) / 255
	g := float64(frame[offset+1]) / 255
	b := float64(frame[offset+2]) / 255
	luma := 0.299*r + 0.587*g + 0.114*b
	iColor := 0.596*r - 0.274*g - 0.322*b
	qColor := 0.211*r - 0.523*g + 0.312*b
	phase := globalIndex & 3
	composite := clamp(luma+0.30*(iColor*chromaCos[phase]+qColor*chromaSin[phase]), 0, 1)
	return 0.75 - 0.625*composite
}

func sourceFieldIndex(outputField uint64) uint64 {
	return outputField * 1001 / 1000
}

func audioSamplesForRFFrame(outputFrame uint64) int {
	start := outputFrame * 8008 / 5
	end := (outputFrame + 1) * 8008 / 5
	return int(end - start)
}

func downmixPair(first, second []int16) []float64 {
	mono := make([]float64, (len(first)+len(second))/2)
	return downmixPairInto(mono, first, second)
}

func downmixPairInto(mono []float64, first, second []int16) []float64 {
	needed := (len(first) + len(second)) / 2
	if len(mono) < needed {
		mono = make([]float64, needed)
	}
	mono = mono[:needed]
	output := 0
	for _, input := range [][]int16{first, second} {
		for index := 0; index < len(input); index += 2 {
			mono[output] = float64(int32(input[index])+int32(input[index+1])) / (2 * 32768)
			output++
		}
	}
	return mono
}

func interpolate(samples []float64, position float64) float64 {
	left := int(position)
	if left >= len(samples)-1 {
		return samples[len(samples)-1]
	}
	fraction := position - float64(left)
	return samples[left] + (samples[left+1]-samples[left])*fraction
}

func phaseIncrement(frequency float64) uint32 {
	return uint32(math.Round(frequency / SampleRate * float64(uint64(1)<<32)))
}

func quantizeQ115(value float32) int16 {
	scaled := value * 32768
	if scaled >= float32(math.MaxInt16) {
		return math.MaxInt16
	}
	if scaled <= float32(math.MinInt16) {
		return math.MinInt16
	}
	if scaled < 0 {
		return int16(scaled - 0.5)
	}
	return int16(scaled + 0.5)
}

func roundInt64(value float64) int64 {
	if value < 0 {
		return int64(value - 0.5)
	}
	return int64(value + 0.5)
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
