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
	StartCID uint16
	EndCID   uint16
	StartUni rune
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

		// 解析 begincodespacerange ... endcodespacerange
		if strings.Contains(line, "begincodespacerange") {
			// codespacerange 主要用于确定输入码的长度，对于解码不是必需的
			// 但我们仍然需要解析它以跳过相关内容
			if err := parseCodeSpaceRange(reader); err != nil {
				return nil, err
			}
		}

		// 检查是否是Identity-H或Identity-V映射
		// 这些是特殊的CMap，CID直接等于Unicode码点
		if strings.Contains(line, "/Identity-H") || strings.Contains(line, "/Identity-V") {
			// Identity映射：CID = Unicode
			// 在这种情况下，我们不需要额外的映射表，因为CID直接就是Unicode码点
			debugPrintf("✓ Detected Identity CMap: %s\n", line)
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
				StartCID: startCIDVal,
				EndCID:   endCIDVal,
				StartUni: startUniVal,
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

// parseCodeSpaceRange 解析 codespacerange（主要用于跳过）
func parseCodeSpaceRange(reader *bufio.Reader) error {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if strings.Contains(line, "endcodespacerange") {
			break
		}

		// 我们不需要实际解析codespacerange的内容
		// 它主要用于确定输入码的长度
	}

	return nil
}

// MapCIDToUnicode 将 CID 映射到 Unicode
func (m *CIDToUnicodeMap) MapCIDToUnicode(cid uint16) (rune, bool) {
	// 首先查找直接映射
	if uni, ok := m.Mappings[cid]; ok {
		// 验证映射的Unicode字符
		if isValidUnicodeRuneForCID(uni) {
			return uni, true
		}
		debugPrintf("⚠️ Invalid Unicode in mapping for CID %d: U+%04X\n", cid, uni)
		return 0, false
	}

	// 然后查找范围映射
	for _, r := range m.Ranges {
		if cid >= r.StartCID && cid <= r.EndCID {
			offset := cid - r.StartCID
			uni := r.StartUni + rune(offset)

			// 验证计算出的Unicode字符
			if isValidUnicodeRuneForCID(uni) {
				return uni, true
			}
			debugPrintf("⚠️ Invalid Unicode in range for CID %d: U+%04X\n", cid, uni)
			return 0, false
		}
	}

	return 0, false
}

// isValidUnicodeRuneForCID 验证Unicode码点是否有效
func isValidUnicodeRuneForCID(r rune) bool {
	// 检查是否是有效的UTF-8 rune
	if r < 0 || r > 0x10FFFF {
		return false
	}
	// 排除代理对范围(U+D800到U+DFFF)
	if r >= 0xD800 && r <= 0xDFFF {
		return false
	}
	return true
}

// MapCIDToUnicodeWithIdentity 将 CID 映射到 Unicode，支持Identity映射
func (m *CIDToUnicodeMap) MapCIDToUnicodeWithIdentity(cid uint16, isIdentity bool) (rune, bool) {
	// 如果是Identity映射，CID直接等于Unicode码点
	if isIdentity {
		return rune(cid), true
	}

	// 否则使用常规映射
	return m.MapCIDToUnicode(cid)
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
