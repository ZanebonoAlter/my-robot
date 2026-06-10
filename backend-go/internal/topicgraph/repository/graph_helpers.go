package repository

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"syntopica-backend/internal/models"
	tagging "syntopica-backend/internal/tagmanagement"

	"gorm.io/gorm"
)

// GetCategoryColor returns the color for a given category.
func GetCategoryColor(category string) string {
	switch category {
	case "event":
		return "#f59e0b" // amber
	case "person":
		return "#10b981" // emerald
	case "keyword":
		return "#6366f1" // indigo
	default:
		return "#6366f1" // default to indigo for unknown categories
	}
}

// sortTagsByScoreMap converts a map of tags to a sorted slice.
func sortTagsByScoreMap(tagMap map[string]*tagging.TopicTag) []tagging.TopicTag {
	result := make([]tagging.TopicTag, 0, len(tagMap))
	for _, tag := range tagMap {
		result = append(result, *tag)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].QualityScore == result[j].QualityScore {
			if result[i].Score == result[j].Score {
				return result[i].Label < result[j].Label
			}
			return result[i].Score > result[j].Score
		}
		return result[i].QualityScore > result[j].QualityScore
	})

	return result
}

func markTopicTagsQuality(tags []tagging.TopicTag) {
	for i := range tags {
		tags[i].IsLowQuality = !tags[i].IsAbstract && tags[i].QualityScore < 0.3
	}
}

func finalizeTopicTagQuality(tagMaps ...map[string]*tagging.TopicTag) {
	for _, tagMap := range tagMaps {
		for _, tag := range tagMap {
			tag.IsLowQuality = !tag.IsAbstract && tag.QualityScore < 0.3
		}
	}
}

// buildGraphPayloadFromArticles builds graph nodes and edges from article tag data.
func buildGraphPayloadFromArticles(db *gorm.DB, data []ArticleTagData) ([]tagging.GraphNode, []tagging.GraphEdge, []tagging.TopicTag, int) {
	topicNodes := map[string]*tagging.GraphNode{}
	feedNodes := map[string]*tagging.GraphNode{}
	edgeMap := map[string]*tagging.GraphEdge{}
	topicScores := map[string]tagging.TopicTag{}
	articleSet := make(map[uint]bool)

	for _, item := range data {
		articleSet[item.ArticleID] = true

		feedNodeID := fmt.Sprintf("feed-%d", item.FeedID)
		if _, exists := feedNodes[feedNodeID]; !exists {
			feedNodes[feedNodeID] = &tagging.GraphNode{
				ID:       feedNodeID,
				Label:    item.FeedTitle,
				Kind:     "feed",
				Weight:   1,
				Color:    item.FeedColor,
				FeedName: item.FeedTitle,
			}
		}
		feedNodes[feedNodeID].Weight += 0.35

		topicSlug := item.TopicTag.Slug
		topicLabel := item.TopicTag.Label
		topicCategory := tagging.NormalizeDisplayCategory(item.TopicTag.Kind, item.TopicTag.Category)

		if _, exists := topicNodes[topicSlug]; !exists {
			topicNodes[topicSlug] = &tagging.GraphNode{
				ID:           topicSlug,
				Label:        topicLabel,
				Slug:         topicSlug,
				Kind:         "topic",
				Category:     topicCategory,
				Icon:         item.TopicTag.Icon,
				Color:        GetCategoryColor(topicCategory),
				Weight:       0,
				ArticleCount: 0,
			}
		}
		topicNodes[topicSlug].Weight += item.Score
		topicNodes[topicSlug].ArticleCount++

		merged := topicScores[topicSlug]
		if merged.Label == "" || merged.Score < item.Score {
			topicScores[topicSlug] = tagging.TopicTag{
				ID:           item.TopicTag.ID,
				Label:        topicLabel,
				Slug:         topicSlug,
				Category:     topicCategory,
				Icon:         item.TopicTag.Icon,
				Kind:         item.TopicTag.Category,
				Score:        item.Score,
				QualityScore: item.TopicTag.QualityScore,
				IsLowQuality: item.TopicTag.Source != "abstract" && item.TopicTag.QualityScore < 0.3,
				Description:  item.TopicTag.Description,
			}
		}

		edgeKey := topicSlug + "::" + feedNodeID
		if _, exists := edgeMap[edgeKey]; !exists {
			edgeMap[edgeKey] = &tagging.GraphEdge{ID: edgeKey, Source: topicSlug, Target: feedNodeID, Kind: "topic_feed", Weight: 0}
		}
		edgeMap[edgeKey].Weight += item.Score
	}

	// Build topic-topic edges from co-occurrence in same article
	articleTopics := make(map[uint][]string)
	for _, item := range data {
		articleTopics[item.ArticleID] = append(articleTopics[item.ArticleID], item.TopicTag.Slug)
	}
	for _, slugs := range articleTopics {
		for i := 0; i < len(slugs); i++ {
			for j := i + 1; j < len(slugs); j++ {
				if slugs[i] == slugs[j] {
					continue
				}
				left, right := slugs[i], slugs[j]
				if left > right {
					left, right = right, left
				}
				edgeKey := left + "::" + right
				if _, exists := edgeMap[edgeKey]; !exists {
					edgeMap[edgeKey] = &tagging.GraphEdge{ID: edgeKey, Source: left, Target: right, Kind: "topic_topic", Weight: 0}
				}
				edgeMap[edgeKey].Weight += 0.5
			}
		}
	}

	// Identify abstract tags (parent tags in topic_tag_relations)
	findAbstractSlugs(db, topicNodes)

	nodes := make([]tagging.GraphNode, 0, len(topicNodes)+len(feedNodes))
	for _, node := range topicNodes {
		nodes = append(nodes, *node)
	}
	for _, node := range feedNodes {
		nodes = append(nodes, *node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Weight == nodes[j].Weight {
			return nodes[i].Label < nodes[j].Label
		}
		return nodes[i].Weight > nodes[j].Weight
	})

	edges := make([]tagging.GraphEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, *edge)
	}
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].Weight > edges[j].Weight })

	topTopics := make([]tagging.TopicTag, 0, len(topicScores))
	for _, topic := range topicScores {
		topic.Score = topicNodes[topic.Slug].Weight
		topTopics = append(topTopics, topic)
	}
	markTopicTagsQuality(topTopics)
	sort.SliceStable(topTopics, func(i, j int) bool {
		if topTopics[i].QualityScore == topTopics[j].QualityScore {
			if topTopics[i].Score == topTopics[j].Score {
				return topTopics[i].Label < topTopics[j].Label
			}
			return topTopics[i].Score > topTopics[j].Score
		}
		return topTopics[i].QualityScore > topTopics[j].QualityScore
	})

	return nodes, edges, topTopics, len(articleSet)
}

// findAbstractSlugs queries topic_tag_relations to identify which tag slugs are abstract parents.
func findAbstractSlugs(db *gorm.DB, topicNodes map[string]*tagging.GraphNode) {
	var abstractParentIDs []uint
	db.Model(&models.TopicTagRelation{}).
		Select("DISTINCT parent_id").
		Pluck("parent_id", &abstractParentIDs)

	if len(abstractParentIDs) == 0 {
		return
	}

	var parentTags []models.TopicTag
	db.Where("id IN ?", abstractParentIDs).Find(&parentTags)

	abstractSlugs := make(map[string]bool, len(parentTags))
	for _, t := range parentTags {
		abstractSlugs[t.Slug] = true
	}

	for slug, node := range topicNodes {
		if abstractSlugs[slug] {
			node.IsAbstract = true
		}
	}
}

// includeAbstractParents adds abstract parent tags that have no direct source='llm' associations.
func includeAbstractParents(db *gorm.DB, tagMaps ...map[string]*tagging.TopicTag) {
	var parentIDs []uint
	db.Model(&models.TopicTagRelation{}).
		Where("relation_type = ?", "abstract").
		Select("DISTINCT parent_id").Pluck("parent_id", &parentIDs)
	if len(parentIDs) == 0 {
		return
	}

	for _, pid := range parentIDs {
		var pt models.TopicTag
		if err := db.First(&pt, pid).Error; err != nil {
			continue
		}
		if pt.Status != "" && pt.Status != "active" {
			continue
		}

		cat := tagging.NormalizeDisplayCategory(pt.Kind, pt.Category)

		var targetMap map[string]*tagging.TopicTag
		switch cat {
		case "event":
			if len(tagMaps) > 0 {
				targetMap = tagMaps[0]
			}
		case "person":
			if len(tagMaps) > 1 {
				targetMap = tagMaps[1]
			}
		default:
			if len(tagMaps) > 2 {
				targetMap = tagMaps[2]
			}
		}
		if targetMap == nil {
			continue
		}

		if _, exists := targetMap[pt.Slug]; !exists {
			targetMap[pt.Slug] = &tagging.TopicTag{
				ID:           pt.ID,
				Label:        pt.Label,
				Slug:         pt.Slug,
				Category:     cat,
				Kind:         pt.Category,
				Icon:         pt.Icon,
				Description:  pt.Description,
				Score:        0,
				QualityScore: pt.QualityScore,
				IsLowQuality: pt.Source != "abstract" && pt.QualityScore < 0.3,
			}
		}
	}
}

// enrichAbstractTags queries topic_tag_relations and enriches tags in the category maps.
func enrichAbstractTags(db *gorm.DB, tagMaps ...map[string]*tagging.TopicTag) {
	var relations []models.TopicTagRelation
	db.Preload("Parent").Preload("Child").Find(&relations)
	if len(relations) == 0 {
		return
	}

	parentToChildren := make(map[uint][]string)
	parentByID := make(map[uint]*models.TopicTag)
	for _, rel := range relations {
		if rel.Parent != nil && rel.Child != nil {
			parentToChildren[rel.ParentID] = append(parentToChildren[rel.ParentID], rel.Child.Slug)
			parentByID[rel.ParentID] = rel.Parent
		}
	}

	abstractSlugs := make(map[string]bool, len(parentByID))
	for _, parent := range parentByID {
		abstractSlugs[parent.Slug] = true
	}

	for _, m := range tagMaps {
		for slug, tag := range m {
			if abstractSlugs[slug] {
				tag.IsAbstract = true
			}
		}
	}

	allSlugs := make(map[string]*tagging.TopicTag)
	for _, m := range tagMaps {
		for slug, tag := range m {
			allSlugs[slug] = tag
		}
	}

	for parentID, childSlugs := range parentToChildren {
		parent, ok := parentByID[parentID]
		if !ok {
			continue
		}
		tag, exists := allSlugs[parent.Slug]
		if !exists {
			continue
		}
		tag.IsAbstract = true
		tag.ChildSlugs = childSlugs
	}
}

func toTitle(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	if len(s) == 0 {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToUpper(r[0])
	result := string(r)
	return strings.ReplaceAll(result, "Ai", "AI")
}

func buildRelatedTopicsFromTags(relatedTags []tagging.RelatedTag, limit int) []tagging.TopicTag {
	result := make([]tagging.TopicTag, 0, len(relatedTags))
	for _, rt := range relatedTags {
		result = append(result, tagging.TopicTag{
			ID:       rt.ID,
			Label:    rt.Label,
			Slug:     rt.Slug,
			Category: rt.Category,
			Kind:     rt.Kind,
			Score:    float64(rt.Cooccurrence),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].Label < result[j].Label
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}
