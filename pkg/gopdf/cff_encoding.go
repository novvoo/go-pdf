package gopdf

import (
	"encoding/binary"
	"errors"
)

func parseCFFEncoding(data []byte) (map[byte]string, error) {
	if len(data) < 4 {
		return nil, errors.New("cff: too short")
	}
	major := data[0]
	hdrSize := int(data[2])
	if major != 1 || hdrSize < 4 || hdrSize > len(data) {
		return nil, errors.New("cff: bad header")
	}

	off := hdrSize
	_, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, err
	}
	off = next

	top, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, err
	}
	if len(top) == 0 {
		return nil, errors.New("cff: missing top dict")
	}
	topDict := top[0]
	off = next

	stringsIndex, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, err
	}
	off = next

	_, next, err = parseCFFIndex(data, off)
	if err != nil {
		return nil, err
	}

	charsetOff, encodingOff, charStringsOff := parseCFFTopDictOffsets(topDict)
	if charStringsOff <= 0 || charStringsOff >= len(data) {
		return nil, errors.New("cff: missing charstrings offset")
	}

	nGlyphs, err := countCFFIndexObjectsAt(data, charStringsOff)
	if err != nil {
		return nil, err
	}
	if nGlyphs <= 0 {
		return nil, errors.New("cff: empty charstrings")
	}

	encoding := make(map[byte]string, 256)

	var gidToSID []uint16
	if charsetOff > 2 && charsetOff < len(data) {
		gidToSID, _ = parseCFFCharset(data, charsetOff, nGlyphs)
	}

	codeToGID, err := parseCFFEncodingMap(data, encodingOff, gidToSID)
	if err != nil || len(codeToGID) == 0 {
		return nil, err
	}

	for code, gid := range codeToGID {
		if gid < 0 || gid >= len(gidToSID) {
			continue
		}
		sid := int(gidToSID[gid])
		name := cffSIDToString(sid, stringsIndex)
		if name == "" {
			continue
		}
		encoding[code] = "/" + name
	}

	if len(encoding) == 0 {
		return nil, errors.New("cff: empty encoding")
	}
	return encoding, nil
}

func parseCFFIndex(data []byte, off int) (objects [][]byte, next int, err error) {
	if off+2 > len(data) {
		return nil, off, errors.New("cff: index truncated")
	}
	count := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	if count == 0 {
		return nil, off, nil
	}
	if off >= len(data) {
		return nil, off, errors.New("cff: index truncated")
	}
	offSize := int(data[off])
	off++
	if offSize < 1 || offSize > 4 {
		return nil, off, errors.New("cff: bad offSize")
	}
	if off+(count+1)*offSize > len(data) {
		return nil, off, errors.New("cff: index offsets truncated")
	}

	offsets := make([]int, count+1)
	for i := 0; i < count+1; i++ {
		offsets[i] = int(readCFFOffset(data[off+i*offSize:off+(i+1)*offSize], offSize))
	}
	off += (count + 1) * offSize

	dataStart := off
	dataLen := offsets[count] - 1
	if dataLen < 0 || dataStart+dataLen > len(data) {
		return nil, off, errors.New("cff: index data truncated")
	}
	dataBlock := data[dataStart : dataStart+dataLen]

	objects = make([][]byte, 0, count)
	for i := 0; i < count; i++ {
		start := offsets[i] - 1
		end := offsets[i+1] - 1
		if start < 0 || end < start || end > len(dataBlock) {
			return nil, off, errors.New("cff: bad object offsets")
		}
		objects = append(objects, dataBlock[start:end])
	}
	return objects, dataStart + dataLen, nil
}

func readCFFOffset(b []byte, size int) uint32 {
	var v uint32
	for i := 0; i < size; i++ {
		v = (v << 8) | uint32(b[i])
	}
	return v
}

func countCFFIndexObjectsAt(data []byte, off int) (int, error) {
	if off+2 > len(data) {
		return 0, errors.New("cff: index truncated")
	}
	return int(binary.BigEndian.Uint16(data[off : off+2])), nil
}

func parseCFFTopDictOffsets(dict []byte) (charsetOff, encodingOff, charStringsOff int) {
	var stack []int
	i := 0
	for i < len(dict) {
		b := dict[i]
		if b <= 21 {
			op := int(b)
			i++
			if op == 12 && i < len(dict) {
				op = 1200 + int(dict[i])
				i++
			}
			if len(stack) > 0 {
				val := stack[len(stack)-1]
				switch op {
				case 15:
					charsetOff = val
				case 16:
					encodingOff = val
				case 17:
					charStringsOff = val
				}
			}
			stack = stack[:0]
			continue
		}
		num, n := readCFFDictNumber(dict[i:])
		stack = append(stack, num)
		i += n
	}
	return charsetOff, encodingOff, charStringsOff
}

func readCFFDictNumber(b []byte) (int, int) {
	if len(b) == 0 {
		return 0, 0
	}
	x := b[0]
	switch {
	case x >= 32 && x <= 246:
		return int(x) - 139, 1
	case x >= 247 && x <= 250:
		if len(b) < 2 {
			return 0, 1
		}
		return (int(x)-247)*256 + int(b[1]) + 108, 2
	case x >= 251 && x <= 254:
		if len(b) < 2 {
			return 0, 1
		}
		return -(int(x)-251)*256 - int(b[1]) - 108, 2
	case x == 28:
		if len(b) < 3 {
			return 0, 1
		}
		return int(int16(binary.BigEndian.Uint16(b[1:3]))), 3
	case x == 29:
		if len(b) < 5 {
			return 0, 1
		}
		return int(int32(binary.BigEndian.Uint32(b[1:5]))), 5
	default:
		return 0, 1
	}
}

func parseCFFCharset(data []byte, off int, nGlyphs int) ([]uint16, error) {
	if off >= len(data) || nGlyphs <= 0 {
		return nil, errors.New("cff: bad charset")
	}
	format := data[off]
	off++
	gidToSID := make([]uint16, nGlyphs)
	gidToSID[0] = 0
	gid := 1

	switch format {
	case 0:
		need := (nGlyphs - 1) * 2
		if off+need > len(data) {
			return gidToSID, errors.New("cff: charset truncated")
		}
		for gid < nGlyphs {
			gidToSID[gid] = binary.BigEndian.Uint16(data[off : off+2])
			off += 2
			gid++
		}
		return gidToSID, nil
	case 1:
		for gid < nGlyphs {
			if off+3 > len(data) {
				return gidToSID, errors.New("cff: charset truncated")
			}
			first := int(binary.BigEndian.Uint16(data[off : off+2]))
			nLeft := int(data[off+2])
			off += 3
			for j := 0; j <= nLeft && gid < nGlyphs; j++ {
				gidToSID[gid] = uint16(first + j)
				gid++
			}
		}
		return gidToSID, nil
	case 2:
		for gid < nGlyphs {
			if off+4 > len(data) {
				return gidToSID, errors.New("cff: charset truncated")
			}
			first := int(binary.BigEndian.Uint16(data[off : off+2]))
			nLeft := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
			off += 4
			for j := 0; j <= nLeft && gid < nGlyphs; j++ {
				gidToSID[gid] = uint16(first + j)
				gid++
			}
		}
		return gidToSID, nil
	default:
		return gidToSID, errors.New("cff: unknown charset format")
	}
}

func parseCFFEncodingMap(data []byte, off int, gidToSID []uint16) (map[byte]int, error) {
	if off <= 1 || off >= len(data) {
		return nil, errors.New("cff: no custom encoding")
	}
	format := data[off]
	hasSupplement := (format & 0x80) != 0
	format = format & 0x7F
	off++

	codeToGID := make(map[byte]int, 256)
	gid := 1

	switch format {
	case 0:
		if off >= len(data) {
			return nil, errors.New("cff: encoding truncated")
		}
		nCodes := int(data[off])
		off++
		if off+nCodes > len(data) {
			return nil, errors.New("cff: encoding truncated")
		}
		for i := 0; i < nCodes; i++ {
			codeToGID[data[off+i]] = gid
			gid++
		}
		off += nCodes
	case 1:
		if off >= len(data) {
			return nil, errors.New("cff: encoding truncated")
		}
		nRanges := int(data[off])
		off++
		for i := 0; i < nRanges; i++ {
			if off+2 > len(data) {
				return nil, errors.New("cff: encoding truncated")
			}
			first := data[off]
			nLeft := int(data[off+1])
			off += 2
			for j := 0; j <= nLeft; j++ {
				codeToGID[first+byte(j)] = gid
				gid++
			}
		}
	default:
		return nil, errors.New("cff: unknown encoding format")
	}

	if hasSupplement {
		if off >= len(data) {
			return codeToGID, nil
		}
		nSups := int(data[off])
		off++
		sidToGID := make(map[int]int, len(gidToSID))
		for g, sid := range gidToSID {
			sidToGID[int(sid)] = g
		}
		for i := 0; i < nSups; i++ {
			if off+3 > len(data) {
				break
			}
			code := data[off]
			sid := int(binary.BigEndian.Uint16(data[off+1 : off+3]))
			off += 3
			if g, ok := sidToGID[sid]; ok {
				codeToGID[code] = g
			}
		}
	}

	return codeToGID, nil
}

func cffSIDToString(sid int, stringsIndex [][]byte) string {
	if sid >= 0 && sid < len(cffStandardStrings) {
		return cffStandardStrings[sid]
	}
	i := sid - 391
	if i < 0 || i >= len(stringsIndex) {
		return ""
	}
	return string(stringsIndex[i])
}
