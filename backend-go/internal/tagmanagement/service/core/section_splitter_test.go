package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// bodyOf repeats seed until the result carries at least minRunes runes.
func bodyOf(seed string, minRunes int) string {
	seedRunes := len([]rune(seed))
	n := (minRunes + seedRunes - 1) / seedRunes
	return strings.Repeat(seed, n)
}

func sectionTitles(sections []Section) []string {
	titles := make([]string, 0, len(sections))
	for _, s := range sections {
		titles = append(titles, s.Title)
	}
	return titles
}

func TestSplitSectionsWeeklyDigestSkipsIntro(t *testing.T) {
	md := "# 科技周刊第100期\n\n" +
		"## 导读\n" + bodyOf("本期导读概述了各栏目要点。", 350) + "\n\n" +
		"## 科技动态\n" + bodyOf("AI缓存技术取得新突破。", 350) + "\n\n" +
		"## 工具\n" + bodyOf("本期推荐三款效率工具。", 350) + "\n\n" +
		"## 言论\n" + bodyOf("开发者讨论提示词缓存。", 350) + "\n\n" +
		"## 图片\n" + bodyOf("本期图片记录发布会现场。", 350)

	sections := splitSections(md)

	require.Len(t, sections, 4)
	require.Equal(t, []string{"科技动态", "工具", "言论", "图片"}, sectionTitles(sections))
	for _, s := range sections {
		require.NotEmpty(t, s.Content)
		require.NotContains(t, s.Content, "导读")
	}
}

func TestSplitSectionsReturnsNilWithoutH2Headings(t *testing.T) {
	require.Nil(t, splitSections("# 只有h1标题\n正文若干行。\n没有栏目结构。"))
	require.Nil(t, splitSections("### 子标题不是栏目边界\n正文内容。\n### 另一个子标题\n更多正文。"))
	require.Nil(t, splitSections(""))
}

func TestSplitSectionsReturnsNilWhenOnlyIntroPresent(t *testing.T) {
	require.Nil(t, splitSections("# 周刊\n## 导读\n"+bodyOf("本期只有导读。", 350)))
}

func TestSplitSectionsMergesAllShortSectionsIntoOne(t *testing.T) {
	md := "# 周刊\n## 栏目一\n" + bodyOf("甲", 100) + "\n\n## 栏目二\n" + bodyOf("乙", 100) + "\n\n## 栏目三\n" + bodyOf("丙", 100)

	sections := splitSections(md)

	require.Len(t, sections, 1)
	require.Contains(t, sections[0].Content, bodyOf("甲", 100))
	require.Contains(t, sections[0].Content, bodyOf("乙", 100))
	require.Contains(t, sections[0].Content, bodyOf("丙", 100))
}

func TestSplitSectionsShortSectionMergesIntoNextSection(t *testing.T) {
	md := "# 周刊\n## 短栏目\n" + bodyOf("短", 100) + "\n\n## 长栏目\n" + bodyOf("长栏目正文。", 350)

	sections := splitSections(md)

	require.Len(t, sections, 1)
	require.Equal(t, "长栏目", sections[0].Title)
	require.Contains(t, sections[0].Content, "## 短栏目")
	require.Contains(t, sections[0].Content, bodyOf("短", 100))
	require.Contains(t, sections[0].Content, bodyOf("长栏目正文。", 350))
}

func TestSplitSectionsCapsAtEightViaTailMerge(t *testing.T) {
	var b strings.Builder
	b.WriteString("# 周刊")
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&b, "\n\n## 栏目%d\n%s", i, bodyOf(fmt.Sprintf("栏目%d的正文内容。", i), 320))
	}

	sections := splitSections(b.String())

	require.Len(t, sections, 8)
	require.Equal(t, []string{"栏目1", "栏目2", "栏目3", "栏目4", "栏目5", "栏目6", "栏目7", "栏目8"}, sectionTitles(sections))
	require.Contains(t, sections[7].Content, "## 栏目9")
	require.Contains(t, sections[7].Content, "## 栏目10")
	require.Contains(t, sections[7].Content, bodyOf("栏目10的正文内容。", 320))
}

func TestSplitSectionsSplitsLongSectionByH3(t *testing.T) {
	md := "# 周刊\n\n## 本期专题\n" +
		"先看本期三个专题的背景。\n\n" +
		"### 专题一\n" + bodyOf("专题一正文详述。", 1200) + "\n\n" +
		"### 专题二\n" + bodyOf("专题二正文详述。", 1200) + "\n\n" +
		"### 专题三\n" + bodyOf("专题三正文详述。", 1200)

	sections := splitSections(md)

	require.Len(t, sections, 3)
	for _, s := range sections {
		require.Equal(t, "本期专题", s.Title)
	}
	require.Contains(t, sections[0].Content, "先看本期三个专题的背景。")
	require.Contains(t, sections[0].Content, "### 专题一")
	require.Contains(t, sections[1].Content, "### 专题二")
	require.Contains(t, sections[2].Content, "### 专题三")
	require.Contains(t, sections[1].Content, bodyOf("专题二正文详述。", 1200))
}

func TestSplitSectionsKeepsLongSectionWithoutH3Intact(t *testing.T) {
	md := "# 周刊\n\n## 超长栏目\n" + bodyOf("没有子标题的超长正文。", 3200)

	sections := splitSections(md)

	require.Len(t, sections, 1)
	require.Equal(t, "超长栏目", sections[0].Title)
	require.Contains(t, sections[0].Content, bodyOf("没有子标题的超长正文。", 3200))
}

func TestSplitSectionsH3LinesAreNotH2Boundaries(t *testing.T) {
	md := "# 标题\n## 栏目\n开头一段。\n### 子标题\n子标题正文。\n#### 更深层级\n更深正文。"

	sections := splitSections(md)

	require.Len(t, sections, 1)
	require.Equal(t, "栏目", sections[0].Title)
	require.Contains(t, sections[0].Content, "### 子标题")
	require.Contains(t, sections[0].Content, "#### 更深层级")
	require.Contains(t, sections[0].Content, "开头一段。")
}
