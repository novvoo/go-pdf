package gopdf

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	popplerdata "github.com/novvoo/go-pdf/poppler-data"
)

// CIDToUnicodeMap CID 到 Unicode 的映射
type CIDToUnicodeMap struct {
	Mappings map[uint16]rune
	Ranges   []cidRange
}

type cidRange struct {
	startCID uint16
	endCID   uint16
	startUni rune
}

// NewCIDToUnicodeMap 创建新的 CID 到 Unicode 映射
func NewCIDToUnicodeMap() *CIDToUnicodeMap {
	return &CIDToUnicodeMap{
		Mappings: make(map[uint16]rune),
		Ranges:   make([]cidRange, 0),
	}
}

// ParseToUnicodeCMap 解析 ToUnicode CMap 流
func ParseToUnicodeCMap(cmapData []byte) (*CIDToUnicodeMap, error) {
	cidMap := NewCIDToUnicodeMap()

	reader := bufio.NewReader(bytes.NewReader(cmapData))

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)

		// 解析 beginbfchar ... endbfchar
		if strings.Contains(line, "beginbfchar") {
			if err := parseBfChar(reader, cidMap); err != nil {
				return nil, err
			}
		}

		// 解析 beginbfrange ... endbfrange
		if strings.Contains(line, "beginbfrange") {
			if err := parseBfRange(reader, cidMap); err != nil {
				return nil, err
			}
		}
	}

	return cidMap, nil
}

// parseBfChar 解析 bfchar 映射
// 格式: <CID> <Unicode>
func parseBfChar(reader *bufio.Reader, cidMap *CIDToUnicodeMap) error {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if strings.Contains(line, "endbfchar") {
			break
		}

		// 解析行: <0001> <4E00>
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		cid := parseHexString(parts[0])
		uni := parseHexString(parts[1])

		if len(cid) >= 2 && len(uni) >= 2 {
			cidVal := uint16(cid[0])<<8 | uint16(cid[1])
			uniVal := rune(uni[0])<<8 | rune(uni[1])
			cidMap.Mappings[cidVal] = uniVal
		}
	}

	return nil
}

// parseBfRange 解析 bfrange 映射
// 格式: <startCID> <endCID> <startUnicode>
func parseBfRange(reader *bufio.Reader, cidMap *CIDToUnicodeMap) error {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if strings.Contains(line, "endbfrange") {
			break
		}

		// 解析行: <0001> <0010> <4E00>
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		startCID := parseHexString(parts[0])
		endCID := parseHexString(parts[1])
		startUni := parseHexString(parts[2])

		if len(startCID) >= 2 && len(endCID) >= 2 && len(startUni) >= 2 {
			startCIDVal := uint16(startCID[0])<<8 | uint16(startCID[1])
			endCIDVal := uint16(endCID[0])<<8 | uint16(endCID[1])
			startUniVal := rune(startUni[0])<<8 | rune(startUni[1])

			cidMap.Ranges = append(cidMap.Ranges, cidRange{
				startCID: startCIDVal,
				endCID:   endCIDVal,
				startUni: startUniVal,
			})
		}
	}

	return nil
}

// parseHexString 解析十六进制字符串 <ABCD> -> []byte{0xAB, 0xCD}
func parseHexString(s string) []byte {
	s = strings.Trim(s, "<>")
	s = strings.ReplaceAll(s, " ", "")

	var result []byte
	for i := 0; i < len(s); i += 2 {
		if i+1 < len(s) {
			var b byte
			fmt.Sscanf(s[i:i+2], "%02x", &b)
			result = append(result, b)
		}
	}

	return result
}

// MapCIDToUnicode 将 CID 映射到 Unicode
func (m *CIDToUnicodeMap) MapCIDToUnicode(cid uint16) (rune, bool) {
	// 首先查找直接映射
	if uni, ok := m.Mappings[cid]; ok {
		return uni, true
	}

	// 然后查找范围映射
	for _, r := range m.Ranges {
		if cid >= r.startCID && cid <= r.endCID {
			offset := cid - r.startCID
			return r.startUni + rune(offset), true
		}
	}

	return 0, false
}

// MapCIDsToUnicode 将 CID 数组映射到 Unicode 字符串
func (m *CIDToUnicodeMap) MapCIDsToUnicode(cids []uint16) string {
	var result strings.Builder

	for _, cid := range cids {
		if uni, ok := m.MapCIDToUnicode(cid); ok {
			result.WriteRune(uni)
		} else {
			// 无法映射，使用占位符
			result.WriteRune('□')
		}
	}

	return result.String()
}

// LoadCIDToUnicodeFromRegistry 从 poppler-data 加载 CID 到 Unicode 映射
// registry: Adobe-GB1, Adobe-CNS1, Adobe-Japan1, Adobe-Korea1
func LoadCIDToUnicodeFromRegistry(registry string) (*CIDToUnicodeMap, error) {
	fs := popplerdata.GetFS()

	// 构建文件路径
	path := fmt.Sprintf("cidToUnicode/%s", registry)

	data, err := fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CID to Unicode map for %s: %w", registry, err)
	}
	defer data.Close()

	// 读取文件内容
	content, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read CID to Unicode map: %w", err)
	}

	// 解析映射
	cidMap := NewCIDToUnicodeMap()

	// poppler-data 的 cidToUnicode 文件格式是简单的文本格式
	// 每行: CID Unicode
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			cid, err := strconv.ParseUint(parts[0], 10, 16)
			if err != nil {
				continue
			}

			uni, err := strconv.ParseInt(parts[1], 0, 32)
			if err != nil {
				continue
			}

			cidMap.Mappings[uint16(cid)] = rune(uni)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse CID to Unicode map: %w", err)
	}

	return cidMap, nil
}

