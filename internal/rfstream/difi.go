// Package rfstream generates a DIFI NTSC-M RF transport stream.
//
// SPDX-License-Identifier: MIT
package rfstream

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	DIFIOUI                  uint32 = 0x6a621e
	DataHeaderBytes                 = 28
	ContextPacketBytes              = 108
	VersionPacketBytes              = 44
	SamplesPerPacket                = 356
	SamplesPerLine                  = 910
	SamplesPerHalfLine              = SamplesPerLine / 2
	SamplesPerField                 = 238_875
	SamplesPerFrame                 = 477_750
	SampleRateNumerator      uint64 = 315_000_000
	SampleRateDenominator    uint64 = 22
	SampleRate                      = float64(SampleRateNumerator) / float64(SampleRateDenominator)
	DIFIProfileSampleRateRaw uint64 = 0x00000da7a65d1746
	DIFIProfileDataFormat    uint64 = (1 << 63) | (1 << 61) | (15 << 44) | (15 << 38) | (15 << 32)
	DefaultRFCenterHz        int64  = 189_000_000
	picosecondsPerSecond     uint64 = 1_000_000_000_000
)

var ErrInvalidIQCount = errors.New("DIFI IQ payload must contain at least one sample")

type IQSample struct {
	I int16
	Q int16
}

type timestamp struct {
	seconds     uint32
	picoseconds uint64
}

func timestampFromTime(value time.Time) timestamp {
	return timestamp{
		seconds:     uint32(value.Unix()),
		picoseconds: uint64(value.Nanosecond()) * 1_000,
	}
}

func timestampAtSample(epoch timestamp, sampleIndex uint64) timestamp {
	seconds, picoseconds := sampleOffset(sampleIndex)
	picoseconds += epoch.picoseconds
	seconds += uint64(epoch.seconds) + picoseconds/picosecondsPerSecond
	return timestamp{
		seconds:     uint32(seconds),
		picoseconds: picoseconds % picosecondsPerSecond,
	}
}

func roundedSamplePicoseconds(sampleIndex uint64) uint64 {
	seconds, picoseconds := sampleOffset(sampleIndex)
	return seconds*picosecondsPerSecond + picoseconds
}

func sampleOffset(sampleIndex uint64) (seconds uint64, picoseconds uint64) {
	// The product remains safe for more than 18 years of continuous samples.
	scaled := sampleIndex * SampleRateDenominator
	seconds = scaled / SampleRateNumerator
	remainder := scaled % SampleRateNumerator
	const quotient = picosecondsPerSecond / SampleRateNumerator
	const residual = picosecondsPerSecond % SampleRateNumerator
	picoseconds = remainder*quotient + (remainder*residual+SampleRateNumerator/2)/SampleRateNumerator
	if picoseconds >= picosecondsPerSecond {
		seconds++
		picoseconds -= picosecondsPerSecond
	}
	return seconds, picoseconds
}

func sampleDuration(sampleCount uint64) time.Duration {
	seconds, picoseconds := sampleOffset(sampleCount)
	return time.Duration(seconds)*time.Second + time.Duration(picoseconds/1_000)*time.Nanosecond
}

func EncodeDataPacket(streamID uint32, sequence uint8, stamp timestamp, samples []IQSample) ([]byte, error) {
	return EncodeDataPacketInto(nil, streamID, sequence, stamp, samples)
}

func EncodeDataPacketInto(destination []byte, streamID uint32, sequence uint8, stamp timestamp, samples []IQSample) ([]byte, error) {
	if len(samples) == 0 {
		return nil, ErrInvalidIQCount
	}
	if len(samples) > 1820 {
		return nil, fmt.Errorf("DIFI IQ payload has %d samples, maximum is 1820", len(samples))
	}
	packetBytes := DataHeaderBytes + len(samples)*4
	if cap(destination) < packetBytes {
		destination = make([]byte, packetBytes)
	} else {
		destination = destination[:packetBytes]
		clear(destination)
	}
	packet := destination
	setHeader(packet, 1, false, sequence, uint16(len(packet)/4))
	setCommon(packet, streamID, 0, 0, stamp)
	for index, sample := range samples {
		binary.BigEndian.PutUint16(packet[DataHeaderBytes+index*4:], uint16(sample.I))
		binary.BigEndian.PutUint16(packet[DataHeaderBytes+index*4+2:], uint16(sample.Q))
	}
	return packet, nil
}

func EncodeContextPacket(streamID uint32, sequence uint8, stamp timestamp, rfCenterHz int64, changed, sampleLoss bool) []byte {
	packet := make([]byte, ContextPacketBytes)
	setHeader(packet, 4, true, sequence, 27)
	setCommon(packet, streamID, 0, 1, stamp)
	if changed {
		binary.BigEndian.PutUint32(packet[28:32], 0xfbb98000)
	} else {
		binary.BigEndian.PutUint32(packet[28:32], 0x7bb98000)
	}
	binary.BigEndian.PutUint32(packet[32:36], 75)
	binary.BigEndian.PutUint64(packet[36:44], uint64(6_000_000)<<20)
	binary.BigEndian.PutUint64(packet[52:60], uint64(rfCenterHz<<20))
	binary.BigEndian.PutUint64(packet[76:84], DIFIProfileSampleRateRaw)
	state := uint32((1 << 18) | (1 << 15))
	if sampleLoss {
		state |= 1 << 12
	}
	binary.BigEndian.PutUint32(packet[96:100], state)
	binary.BigEndian.PutUint64(packet[100:108], DIFIProfileDataFormat)
	return packet
}

func EncodeVersionPacket(streamID uint32, sequence uint8, stamp timestamp, now time.Time, changed bool) []byte {
	packet := make([]byte, VersionPacketBytes)
	setHeader(packet, 4, true, sequence, 11)
	setCommon(packet, streamID, 1, 4, stamp)
	if changed {
		binary.BigEndian.PutUint32(packet[28:32], 0x80000002)
	} else {
		binary.BigEndian.PutUint32(packet[28:32], 2)
	}
	binary.BigEndian.PutUint32(packet[32:36], 0x0c)
	binary.BigEndian.PutUint32(packet[36:40], 4)
	year := now.UTC().Year() - 2000
	if year < 0 {
		year = 0
	} else if year > 127 {
		year = 127
	}
	versionInfo := uint32(year)<<25 | uint32(now.UTC().YearDay())<<16 | 1<<10 | 1<<6 | 1
	binary.BigEndian.PutUint32(packet[40:44], versionInfo)
	return packet
}

func setHeader(packet []byte, packetType uint32, tsm bool, sequence uint8, words uint16) {
	word := packetType<<28 | 1<<27 | 3<<22 | 2<<20 | uint32(sequence&0x0f)<<16 | uint32(words)
	if tsm {
		word |= 1 << 24
	}
	binary.BigEndian.PutUint32(packet[:4], word)
}

func setCommon(packet []byte, streamID uint32, informationClass, packetClass uint16, stamp timestamp) {
	binary.BigEndian.PutUint32(packet[4:8], streamID)
	binary.BigEndian.PutUint32(packet[8:12], DIFIOUI)
	binary.BigEndian.PutUint16(packet[12:14], informationClass)
	binary.BigEndian.PutUint16(packet[14:16], packetClass)
	binary.BigEndian.PutUint32(packet[16:20], stamp.seconds)
	binary.BigEndian.PutUint64(packet[20:28], stamp.picoseconds)
}
