package gopdf

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"unicode/utf16"

	popplerdata "github.com/novvoo/go-pdf/poppler-data"
)

// CIDToUnicodeMap CID 到 Unicode 的映射
type CIDToUnicodeMap struct {
	Mappings map[uint16]rune
	Ranges   []cidRange
	parsed   *toUnicodeCMap
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
	cm, err := parseToUnicodeCMapBytes(cmapData)
	if err != nil {
		return nil, err
	}
	cidMap.parsed = cm
	for k, v := range cm.mapping {
		if len(k) != 2 {
			continue
		}
		b := []byte(k)
		cidVal := uint16(b[0])<<8 | uint16(b[1])
		r := []rune(v)
		if len(r) == 1 && isValidUnicodeRuneForCID(r[0]) {
			cidMap.Mappings[cidVal] = r[0]
		}
	}
	return cidMap, nil
}

func parseToUnicodeCMapInternal(cmapData []byte, visited map[string]bool, depth int) (*CIDToUnicodeMap, error) {
	if depth > 6 {
		return NewCIDToUnicodeMap(), nil
	}

	cidMap := NewCIDToUnicodeMap()

	cmapData = bytes.ReplaceAll(cmapData, []byte{'\r'}, []byte{'\n'})
	reader := bufio.NewReader(bytes.NewReader(cmapData))

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if len(line) == 0 {
					break
				}
			} else {
				return nil, err
			}
		}

		line = strings.TrimSpace(line)
		if line == "" {
			if err == io.EOF {
				break
			}
			continue
		}

		if strings.Contains(line, "usecmap") {
			if name := extractUseCMapName(line); name != "" && !visited[name] {
				visited[name] = true
				if baseData, ok := loadCMapFromPopplerData(name); ok {
					baseMap, err := parseToUnicodeCMapInternal(baseData, visited, depth+1)
					if err == nil && baseMap != nil {
						mergeCIDToUnicodeMaps(cidMap, baseMap)
					}
				}
			}
		}

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
			if err := parseCodeSpaceRange(reader); err != nil {
				return nil, err
			}
		}

		if strings.Contains(line, "/Identity-H") || strings.Contains(line, "/Identity-V") {
			debugPrintf("✓ Detected Identity CMap: %s\n", line)
		}

		if err == io.EOF {
			break
		}
	}

	return cidMap, nil
}

func extractUseCMapName(line string) string {
	parts := strings.Fields(line)
	for i := 0; i < len(parts); i++ {
		if parts[i] != "usecmap" {
			continue
		}
		if i == 0 {
			return ""
		}
		name := strings.TrimSpace(parts[i-1])
		name = strings.TrimPrefix(name, "/")
		return name
	}
	return ""
}

func loadCMapFromPopplerData(cmapName string) ([]byte, bool) {
	cmapName = strings.TrimSpace(strings.TrimPrefix(cmapName, "/"))
	if cmapName == "" {
		return nil, false
	}

	fs0 := popplerdata.GetFS()
	matches, _ := fs.Glob(fs0, "cMap/*/"+cmapName)
	if len(matches) == 0 {
		return nil, false
	}

	f, err := fs0.Open(matches[0])
	if err != nil {
		return nil, false
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}

func mergeCIDToUnicodeMaps(dst, src *CIDToUnicodeMap) {
	if dst == nil || src == nil {
		return
	}
	if dst.Mappings == nil {
		dst.Mappings = make(map[uint16]rune, len(src.Mappings))
	}
	for k, v := range src.Mappings {
		if _, exists := dst.Mappings[k]; !exists {
			dst.Mappings[k] = v
		}
	}
	if len(src.Ranges) > 0 {
		dst.Ranges = append(dst.Ranges, src.Ranges...)
	}
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

		toks := extractAllAngleHexTokens(line)
		for len(toks) < 2 {
			next, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			n := strings.TrimSpace(next)
			if strings.Contains(n, "endbfchar") {
				return nil
			}
			toks = append(toks, extractAllAngleHexTokens(n)...)
			if len(toks) >= 2 {
				break
			}
		}
		for i := 0; i+1 < len(toks); i += 2 {
			cid := parseHexString(toks[i])
			uni := parseHexString(toks[i+1])

			if len(cid) >= 2 && len(uni) >= 2 {
				cidVal := uint16(cid[0])<<8 | uint16(cid[1])
				if runes := decodeUTF16BERunes(uni); len(runes) > 0 {
					cidMap.Mappings[cidVal] = runes[0]
				}
			}
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

		toks := extractAllAngleHexTokens(line)
		if len(toks) < 2 {
			continue
		}

		startCID := parseHexString(toks[0])
		endCID := parseHexString(toks[1])
		if len(startCID) < 2 || len(endCID) < 2 {
			continue
		}

		startCIDVal := uint16(startCID[0])<<8 | uint16(startCID[1])
		endCIDVal := uint16(endCID[0])<<8 | uint16(endCID[1])
		if endCIDVal < startCIDVal {
			continue
		}

		isArray := strings.Contains(line, "[")
		uniTokens := append([]string(nil), toks[2:]...)
		if len(uniTokens) == 0 {
			for {
				next, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				n := strings.TrimSpace(next)
				if strings.Contains(n, "endbfrange") {
					return nil
				}
				if strings.Contains(n, "[") {
					isArray = true
				}
				uniTokens = append(uniTokens, extractAllAngleHexTokens(n)...)
				if isArray {
					if strings.Contains(n, "]") {
						break
					}
					continue
				}
				if len(uniTokens) > 0 {
					break
				}
			}
		} else if strings.Contains(line, "]") {
			isArray = true
		} else if isArray {
			for {
				next, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				n := strings.TrimSpace(next)
				uniTokens = append(uniTokens, extractAllAngleHexTokens(n)...)
				if strings.Contains(n, "]") {
					break
				}
			}
		}

		if isArray {
			expected := int(endCIDVal-startCIDVal) + 1
			if len(uniTokens) < expected {
				expected = len(uniTokens)
			}
			for i := 0; i < expected; i++ {
				uniBytes := parseHexString(uniTokens[i])
				if runes := decodeUTF16BERunes(uniBytes); len(runes) > 0 {
					cidMap.Mappings[startCIDVal+uint16(i)] = runes[0]
				}
			}
			continue
		}

		if len(uniTokens) == 0 {
			continue
		}
		startUni := parseHexString(uniTokens[0])
		if len(startUni) < 2 {
			continue
		}
		startRunes := decodeUTF16BERunes(startUni)
		if len(startRunes) == 0 {
			continue
		}

		cidMap.Ranges = append(cidMap.Ranges, cidRange{
			StartCID: startCIDVal,
			EndCID:   endCIDVal,
			StartUni: startRunes[0],
		})
	}

	return nil
}

func extractAllAngleHexTokens(s string) []string {
	out := []string(nil)
	for {
		i := strings.IndexByte(s, '<')
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i:], '>')
		if j < 0 {
			break
		}
		j += i
		hex := strings.ReplaceAll(s[i+1:j], " ", "")
		if hex != "" {
			out = append(out, "<"+hex+">")
		}
		s = s[j+1:]
	}
	return out
}

func decodeUTF16BERunes(b []byte) []rune {
	if len(b) == 0 {
		return nil
	}
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		b = b[2:]
	}
	if len(b) == 0 {
		return nil
	}

	u16s := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u16s = append(u16s, uint16(b[i])<<8|uint16(b[i+1]))
	}

	runes := make([]rune, 0, len(u16s))
	for i := 0; i < len(u16s); i++ {
		u := u16s[i]
		if u >= 0xD800 && u <= 0xDBFF && i+1 < len(u16s) {
			lo := u16s[i+1]
			if lo >= 0xDC00 && lo <= 0xDFFF {
				runes = append(runes, utf16.DecodeRune(rune(u), rune(lo)))
				i++
				continue
			}
		}
		runes = append(runes, rune(u))
	}
	return runes
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
	for i := len(m.Ranges) - 1; i >= 0; i-- {
		r := m.Ranges[i]
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
