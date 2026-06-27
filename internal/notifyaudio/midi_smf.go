package notifyaudio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
)

type smfEventKind uint8

const (
	smfEventShort smfEventKind = iota
	smfEventTempo
)

type smfSequence struct {
	division uint16
	events   []smfEvent
}

type smfEvent struct {
	tick                  uint64
	order                 int
	kind                  smfEventKind
	status                byte
	data1                 byte
	data2                 byte
	tempoMicrosPerQuarter int
}

func parseSMF(data []byte) (smfSequence, error) {
	var seq smfSequence
	if len(data) < 14 || !bytes.Equal(data[:4], []byte("MThd")) {
		return seq, fmt.Errorf("midi file is missing MThd header")
	}
	headerLen := int(binary.BigEndian.Uint32(data[4:8]))
	if headerLen < 6 || len(data) < 8+headerLen {
		return seq, fmt.Errorf("midi header is truncated")
	}
	format := binary.BigEndian.Uint16(data[8:10])
	trackCount := int(binary.BigEndian.Uint16(data[10:12]))
	seq.division = binary.BigEndian.Uint16(data[12:14])
	if trackCount <= 0 {
		return seq, fmt.Errorf("midi file has no tracks")
	}
	if format > 1 {
		return seq, fmt.Errorf("unsupported midi format %d", format)
	}
	pos := 8 + headerLen
	order := 0
	for track := 0; track < trackCount; track++ {
		if pos+8 > len(data) || !bytes.Equal(data[pos:pos+4], []byte("MTrk")) {
			return seq, fmt.Errorf("midi track %d is missing MTrk header", track)
		}
		trackLen := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		pos += 8
		if trackLen < 0 || pos+trackLen > len(data) {
			return seq, fmt.Errorf("midi track %d is truncated", track)
		}
		events, nextOrder, err := parseSMFTrack(data[pos:pos+trackLen], order)
		if err != nil {
			return seq, fmt.Errorf("midi track %d: %w", track, err)
		}
		seq.events = append(seq.events, events...)
		order = nextOrder
		pos += trackLen
	}
	sort.SliceStable(seq.events, func(i, j int) bool {
		if seq.events[i].tick != seq.events[j].tick {
			return seq.events[i].tick < seq.events[j].tick
		}
		return seq.events[i].order < seq.events[j].order
	})
	return seq, nil
}

func parseSMFTrack(data []byte, order int) ([]smfEvent, int, error) {
	var events []smfEvent
	var tick uint64
	var runningStatus byte
	pos := 0
	for pos < len(data) {
		delta, next, err := readSMFVarLen(data, pos)
		if err != nil {
			return nil, order, err
		}
		pos = next
		tick += delta
		if pos >= len(data) {
			return nil, order, fmt.Errorf("event status is truncated")
		}
		status := data[pos]
		if status < 0x80 {
			if runningStatus == 0 {
				return nil, order, fmt.Errorf("running-status event has no previous status")
			}
			status = runningStatus
		} else {
			pos++
			if status < 0xF0 {
				runningStatus = status
			}
		}
		switch {
		case status < 0xF0:
			dataLen := smfChannelDataLen(status)
			if dataLen == 0 {
				return nil, order, fmt.Errorf("unsupported channel status 0x%X", status)
			}
			data1, data2 := byte(0), byte(0)
			if pos >= len(data) {
				return nil, order, fmt.Errorf("channel event data is truncated")
			}
			data1 = data[pos]
			pos++
			if data1 >= 0x80 {
				return nil, order, fmt.Errorf("invalid channel data byte 0x%X", data1)
			}
			if dataLen == 2 {
				if pos >= len(data) {
					return nil, order, fmt.Errorf("channel event second data byte is truncated")
				}
				data2 = data[pos]
				pos++
				if data2 >= 0x80 {
					return nil, order, fmt.Errorf("invalid channel data byte 0x%X", data2)
				}
			}
			events = append(events, smfEvent{
				tick:   tick,
				order:  order,
				kind:   smfEventShort,
				status: status,
				data1:  data1,
				data2:  data2,
			})
			order++
		case status == 0xFF:
			if pos >= len(data) {
				return nil, order, fmt.Errorf("meta event type is truncated")
			}
			metaType := data[pos]
			pos++
			length, next, err := readSMFVarLen(data, pos)
			if err != nil {
				return nil, order, err
			}
			pos = next
			if length > uint64(len(data)-pos) {
				return nil, order, fmt.Errorf("meta event payload is truncated")
			}
			payload := data[pos : pos+int(length)]
			pos += int(length)
			if metaType == 0x51 && len(payload) == 3 {
				tempo := int(payload[0])<<16 | int(payload[1])<<8 | int(payload[2])
				if tempo > 0 {
					events = append(events, smfEvent{
						tick:                  tick,
						order:                 order,
						kind:                  smfEventTempo,
						tempoMicrosPerQuarter: tempo,
					})
					order++
				}
			}
			if metaType == 0x2F {
				return events, order, nil
			}
		case status == 0xF0 || status == 0xF7:
			length, next, err := readSMFVarLen(data, pos)
			if err != nil {
				return nil, order, err
			}
			pos = next
			if length > uint64(len(data)-pos) {
				return nil, order, fmt.Errorf("sysex event payload is truncated")
			}
			pos += int(length)
		default:
			dataLen := smfSystemDataLen(status)
			if pos+dataLen > len(data) {
				return nil, order, fmt.Errorf("system event data is truncated")
			}
			pos += dataLen
		}
	}
	return events, order, nil
}

func readSMFVarLen(data []byte, pos int) (uint64, int, error) {
	var value uint64
	for i := 0; i < 4; i++ {
		if pos >= len(data) {
			return 0, pos, fmt.Errorf("variable-length value is truncated")
		}
		b := data[pos]
		pos++
		value = (value << 7) | uint64(b&0x7F)
		if b < 0x80 {
			return value, pos, nil
		}
	}
	return 0, pos, fmt.Errorf("variable-length value is too long")
}

func smfChannelDataLen(status byte) int {
	switch status & 0xF0 {
	case 0x80, 0x90, 0xA0, 0xB0, 0xE0:
		return 2
	case 0xC0, 0xD0:
		return 1
	default:
		return 0
	}
}

func smfSystemDataLen(status byte) int {
	switch status {
	case 0xF1, 0xF3:
		return 1
	case 0xF2:
		return 2
	default:
		return 0
	}
}
