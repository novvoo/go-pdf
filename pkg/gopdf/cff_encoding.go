package gopdf

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
)

func parseCFFEncoding(data []byte) (map[byte]string, map[byte]uint16, [6]float64, bool, error) {
	var fontMatrix [6]float64
	if len(data) < 4 {
		return nil, nil, fontMatrix, false, errors.New("cff: too short")
	}
	major := data[0]
	hdrSize := int(data[2])
	if major != 1 || hdrSize < 4 || hdrSize > len(data) {
		return nil, nil, fontMatrix, false, errors.New("cff: bad header")
	}

	off := hdrSize
	_, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, nil, fontMatrix, false, err
	}
	off = next

	top, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, nil, fontMatrix, false, err
	}
	if len(top) == 0 {
		return nil, nil, fontMatrix, false, errors.New("cff: missing top dict")
	}
	topDict := top[0]
	off = next

	stringsIndex, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, nil, fontMatrix, false, err
	}
	off = next

	_, _, err = parseCFFIndex(data, off)
	if err != nil {
		return nil, nil, fontMatrix, false, err
	}

	charsetOff, encodingOff, charStringsOff, fontMatrix, hasFontMatrix := parseCFFTopDictOffsets(topDict)
	if charStringsOff <= 0 || charStringsOff >= len(data) {
		return nil, nil, fontMatrix, false, errors.New("cff: missing charstrings offset")
	}

	nGlyphs, err := countCFFIndexObjectsAt(data, charStringsOff)
	if err != nil {
		return nil, nil, fontMatrix, false, err
	}
	if nGlyphs <= 0 {
		return nil, nil, fontMatrix, false, errors.New("cff: empty charstrings")
	}

	encoding := make(map[byte]string, 256)
	codeToGIDU16 := make(map[byte]uint16, 256)

	var gidToSID []uint16
	if charsetOff > 2 && charsetOff < len(data) {
		gidToSID, _ = parseCFFCharset(data, charsetOff, nGlyphs)
	}

	codeToGID, err := parseCFFEncodingMap(data, encodingOff, gidToSID)
	if err != nil || len(codeToGID) == 0 {
		return nil, nil, fontMatrix, hasFontMatrix, err
	}

	for code, gid := range codeToGID {
		if gid >= 0 && gid <= 0xFFFF {
			codeToGIDU16[code] = uint16(gid)
		}
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
		return nil, nil, fontMatrix, hasFontMatrix, errors.New("cff: empty encoding")
	}
	return encoding, codeToGIDU16, fontMatrix, hasFontMatrix, nil
}

func parseCFFCIDToGIDMap(data []byte) ([]uint16, [6]float64, bool, error) {
	var fontMatrix [6]float64
	if len(data) < 4 {
		return nil, fontMatrix, false, errors.New("cff: too short")
	}
	major := data[0]
	hdrSize := int(data[2])
	if major != 1 || hdrSize < 4 || hdrSize > len(data) {
		return nil, fontMatrix, false, errors.New("cff: bad header")
	}

	off := hdrSize
	_, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, fontMatrix, false, err
	}
	off = next

	top, next, err := parseCFFIndex(data, off)
	if err != nil {
		return nil, fontMatrix, false, err
	}
	if len(top) == 0 {
		return nil, fontMatrix, false, errors.New("cff: missing top dict")
	}
	topDict := top[0]
	off = next

	_, next, err = parseCFFIndex(data, off)
	if err != nil {
		return nil, fontMatrix, false, err
	}
	off = next

	_, next, err = parseCFFIndex(data, off)
	if err != nil {
		return nil, fontMatrix, false, err
	}
	_ = next

	charsetOff, _, charStringsOff, fontMatrix, hasFontMatrix := parseCFFTopDictOffsets(topDict)
	if charStringsOff <= 0 || charStringsOff >= len(data) {
		return nil, fontMatrix, hasFontMatrix, errors.New("cff: missing charstrings offset")
	}

	nGlyphs, err := countCFFIndexObjectsAt(data, charStringsOff)
	if err != nil {
		return nil, fontMatrix, hasFontMatrix, err
	}
	if nGlyphs <= 0 {
		return nil, fontMatrix, hasFontMatrix, errors.New("cff: empty charstrings")
	}

	gidToCID, err := parseCFFCharset(data, charsetOff, nGlyphs)
	if err != nil {
		return nil, fontMatrix, hasFontMatrix, err
	}

	maxCID := 0
	for _, cid := range gidToCID {
		if int(cid) > maxCID {
			maxCID = int(cid)
		}
	}
	if maxCID <= 0 {
		return nil, fontMatrix, hasFontMatrix, errors.New("cff: empty charset")
	}

	cidToGID := make([]uint16, maxCID+1)
	for gid, cid := range gidToCID {
		if int(cid) < len(cidToGID) {
			cidToGID[cid] = uint16(gid)
		}
	}
	return cidToGID, fontMatrix, hasFontMatrix, nil
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

func parseCFFTopDictOffsets(dict []byte) (charsetOff, encodingOff, charStringsOff int, fontMatrix [6]float64, hasFontMatrix bool) {
	var stack []float64
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
			switch op {
			case 15:
				if len(stack) > 0 {
					charsetOff = int(stack[len(stack)-1])
				}
			case 16:
				if len(stack) > 0 {
					encodingOff = int(stack[len(stack)-1])
				}
			case 17:
				if len(stack) > 0 {
					charStringsOff = int(stack[len(stack)-1])
				}
			case 1207:
				if len(stack) >= 6 {
					copy(fontMatrix[:], stack[len(stack)-6:])
					hasFontMatrix = true
				}
			}
			stack = stack[:0]
			continue
		}
		num, n := readCFFDictNumberFloat(dict[i:])
		stack = append(stack, num)
		i += n
	}
	return charsetOff, encodingOff, charStringsOff, fontMatrix, hasFontMatrix
}

func readCFFDictNumberFloat(b []byte) (float64, int) {
	if len(b) == 0 {
		return 0, 0
	}
	x := b[0]
	switch {
	case x >= 32 && x <= 246:
		return float64(int(x) - 139), 1
	case x >= 247 && x <= 250:
		if len(b) < 2 {
			return 0, 1
		}
		return float64((int(x)-247)*256 + int(b[1]) + 108), 2
	case x >= 251 && x <= 254:
		if len(b) < 2 {
			return 0, 1
		}
		return float64(-(int(x)-251)*256 - int(b[1]) - 108), 2
	case x == 28:
		if len(b) < 3 {
			return 0, 1
		}
		return float64(int(int16(binary.BigEndian.Uint16(b[1:3])))), 3
	case x == 29:
		if len(b) < 5 {
			return 0, 1
		}
		return float64(int(int32(binary.BigEndian.Uint32(b[1:5])))), 5
	case x == 30:
		s, n := readCFFRealNumberString(b)
		if n <= 0 {
			return 0, 1
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, n
		}
		return f, n
	case x == 255:
		if len(b) < 5 {
			return 0, 1
		}
		v := int32(binary.BigEndian.Uint32(b[1:5]))
		return float64(v) / 65536.0, 5
	default:
		return 0, 1
	}
}

func readCFFRealNumberString(b []byte) (string, int) {
	if len(b) == 0 || b[0] != 30 {
		return "", 0
	}
	var sb strings.Builder
	i := 1
	for i < len(b) {
		hi := b[i] >> 4
		lo := b[i] & 0x0F
		i++
		if !appendCFFRealNibble(&sb, hi) {
			break
		}
		if !appendCFFRealNibble(&sb, lo) {
			break
		}
	}
	return sb.String(), i
}

func appendCFFRealNibble(sb *strings.Builder, n byte) bool {
	switch n {
	case 0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9:
		sb.WriteByte('0' + n)
		return true
	case 0xA:
		sb.WriteByte('.')
		return true
	case 0xB:
		sb.WriteByte('E')
		return true
	case 0xC:
		sb.WriteString("E-")
		return true
	case 0xE:
		sb.WriteByte('-')
		return true
	case 0xF:
		return false
	default:
		return true
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
