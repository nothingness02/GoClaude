package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/GoClaude/common"
)

type Skill struct {
	Meta map[string]string
	Body string
	Path string
}

type SkillLoader struct {
	Skills map[string]Skill
}

func NewSkillLoader() *SkillLoader {
	sl := &SkillLoader{Skills: make(map[string]Skill)}
	sl.LoadAll()
	return sl
}

func (sl *SkillLoader) LoadAll() {
	if _, err := os.Stat(common.SkillsDir); os.IsNotExist(err) {
		return
	}

	filepath.WalkDir(common.SkillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		meta, body := sl.ParseFrontmatter(string(content))
		name := meta["name"]
		if name == "" {
			name = filepath.Base(filepath.Dir(path))
		}

		sl.Skills[name] = Skill{
			Meta: meta,
			Body: body,
			Path: path,
		}
		return nil
	})
}

// 解析 YAML Frontmatter (--- ... ---)
func (sl *SkillLoader) ParseFrontmatter(text string) (map[string]string, string) {
	re := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)`)
	matches := re.FindStringSubmatch(text)

	meta := make(map[string]string)
	if len(matches) != 3 {
		return meta, text
	}

	lines := strings.Split(matches[1], "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			meta[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return meta, strings.TrimSpace(matches[2])
}

// 第一层：向系统 Prompt 注入技能描述
func (sl *SkillLoader) GetDescriptions() string {
	if len(sl.Skills) == 0 {
		return "(no skills available)"
	}
	var lines []string
	for name, skill := range sl.Skills {
		desc := skill.Meta["description"]
		if desc == "" {
			desc = "No description"
		}
		line := fmt.Sprintf("  - %s: %s", name, desc)
		if tags, ok := skill.Meta["tags"]; ok {
			line += fmt.Sprintf(" [%s]", tags)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// 第二层：按需将技能详情加载到 Context
func (sl *SkillLoader) GetContent(name string) string {
	skill, exists := sl.Skills[name]
	if !exists {
		var available []string
		for k := range sl.Skills {
			available = append(available, k)
		}
		return fmt.Sprintf("Error: Unknown skill '%s'. Available: %s", name, strings.Join(available, ", "))
	}
	return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", name, skill.Body)
}

var GlobalSkillLoader = NewSkillLoader()
