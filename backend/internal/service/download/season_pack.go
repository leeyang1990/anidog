package download

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/anidog/anidog-go/internal/service/titleparse"
)

var (
	packEpisodeSxxExx = regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})`)
	packEpisodeToken  = regexp.MustCompile(`(?:^|[\[\]() _.-])(\d{1,3})(?:v\d+)?(?:[\]\]() _.-]|$)`)
)

// mapSeasonPackMediaFiles 把合集目录中的视频映射到剧集号。只接受明确可识别
// 的文件名；宁可让未识别的单集回到后续逐集补齐，也不能按文件排序盲猜集号。
func mapSeasonPackMediaFiles(contentPath string, start, end int) (map[int]string, error) {
	if start <= 0 || end < start {
		return nil, fmt.Errorf("无效合集范围 %d-%d", start, end)
	}
	info, err := os.Stat(contentPath)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	if info.IsDir() {
		err = filepath.WalkDir(contentPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && isRaceMediaExtension(filepath.Ext(path)) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else if isRaceMediaExtension(filepath.Ext(contentPath)) {
		paths = append(paths, contentPath)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("合集内没有可识别视频文件")
	}

	mapped := make(map[int]string)
	unassignedExtras := make([]string, 0)
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		ep := packEpisodeNumber(name, start, end)
		if ep == 0 {
			if isExtraEpisodeName(name) {
				unassignedExtras = append(unassignedExtras, path)
			}
			continue
		}
		if previous, exists := mapped[ep]; !exists || fileSize(path) > fileSize(previous) {
			mapped[ep] = path
		}
	}

	// 标题声明 01-24+OVA 时，Orchestrator 会把范围扩到第 25 集。把明确
	// 标记为 OVA/OAD/OAV 的额外视频分配给范围末尾尚未覆盖的集数。
	sort.Strings(unassignedExtras)
	for _, path := range unassignedExtras {
		for ep := end; ep >= start; ep-- {
			if _, exists := mapped[ep]; !exists {
				mapped[ep] = path
				break
			}
		}
	}

	rangeSize := end - start + 1
	minimum := (rangeSize*60 + 99) / 100
	if minimum < 4 {
		minimum = 4
	}
	if len(mapped) < minimum {
		return nil, fmt.Errorf("合集文件只能识别 %d/%d 集，低于安全阈值", len(mapped), rangeSize)
	}
	return mapped, nil
}

func packEpisodeNumber(name string, start, end int) int {
	parsed := titleparse.Parse(name)
	if parsed != nil && parsed.EpisodeNum != nil && *parsed.EpisodeNum >= start && *parsed.EpisodeNum <= end {
		return *parsed.EpisodeNum
	}
	for _, re := range []*regexp.Regexp{packEpisodeSxxExx, packEpisodeToken} {
		matches := re.FindAllStringSubmatch(name, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			ep, _ := strconv.Atoi(match[1])
			if ep >= start && ep <= end && ep != 720 && ep != 1080 && ep != 2160 {
				return ep
			}
		}
	}
	return 0
}

func isExtraEpisodeName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "ova") || strings.Contains(lower, "oad") || strings.Contains(lower, "oav")
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
