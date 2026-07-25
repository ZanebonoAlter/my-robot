package aisettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
)

const openNotebookConfigKey = "open_notebook_config"
const firecrawlConfigKey = "firecrawl_config"
const rsshubConfigKey = "rsshub_config"
const dailyReportTimeKey = "daily_report_time"
const defaultDailyReportTime = "21:00"
const boardUpgradeSuggestTimeKey = "semantic_board_upgrade_suggest_time"
const defaultBoardUpgradeSuggestTime = "06:30"

var hhmmPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

func loadConfigByKey(key string) (map[string]interface{}, *models.AISettings, error) {
	var settings models.AISettings
	err := database.DB.Where("key = ?", key).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[string]interface{}{}, nil, nil
		}
		return nil, nil, err
	}

	config := map[string]interface{}{}
	if settings.Value == "" {
		return config, &settings, nil
	}

	if err := json.Unmarshal([]byte(settings.Value), &config); err != nil {
		return nil, nil, err
	}

	return config, &settings, nil

}

func saveConfigByKey(key string, config map[string]interface{}, description string) error {
	configJSON, err := models.ToJSONValue(config)
	if err != nil {
		return err
	}

	var settings models.AISettings
	dbErr := database.DB.Where("key = ?", key).First(&settings).Error
	if dbErr == nil {
		settings.Value = configJSON
		if description != "" {
			settings.Description = description
		}
		return database.DB.Save(&settings).Error
	}

	if !errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return dbErr
	}

	settings = models.AISettings{
		Key:         key,
		Value:       configJSON,
		Description: description,
	}

	return database.DB.Create(&settings).Error
}

func LoadFirecrawlConfig() (map[string]interface{}, *models.AISettings, error) {
	return loadConfigByKey(firecrawlConfigKey)
}

func SaveFirecrawlConfig(config map[string]interface{}, description string) error {
	return saveConfigByKey(firecrawlConfigKey, config, description)
}

func LoadOpenNotebookConfig() (map[string]interface{}, *models.AISettings, error) {
	return loadConfigByKey(openNotebookConfigKey)
}

func SaveOpenNotebookConfig(config map[string]interface{}, description string) error {
	return saveConfigByKey(openNotebookConfigKey, config, description)
}

// LoadRSSHubConfig 读取 rsshub_config（RSSHub 实例地址等，design E）。
func LoadRSSHubConfig() (map[string]interface{}, *models.AISettings, error) {
	return loadConfigByKey(rsshubConfigKey)
}

// SaveRSSHubConfig 写入 rsshub_config。
func SaveRSSHubConfig(config map[string]interface{}, description string) error {
	return saveConfigByKey(rsshubConfigKey, config, description)
}

// LoadDailyReportTimeConfig loads the daily_report_time setting from ai_settings.
// Returns the HH:MM string. If the key is missing or invalid, returns default "21:00".
func LoadDailyReportTimeConfig() (string, error) {
	var settings models.AISettings
	err := database.DB.Where("key = ?", dailyReportTimeKey).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultDailyReportTime, nil
		}
		return "", err
	}

	value := strings.TrimSpace(settings.Value)
	if !hhmmPattern.MatchString(value) {
		logging.Warnf("Invalid daily_report_time value %q, falling back to default %s", value, defaultDailyReportTime)
		return defaultDailyReportTime, nil
	}
	return value, nil
}

// SaveDailyReportTimeConfig saves the daily_report_time setting.
// Validates HH:MM format (00:00–23:59). Returns error for invalid values.
func SaveDailyReportTimeConfig(value string) error {
	value = strings.TrimSpace(value)
	if !hhmmPattern.MatchString(value) {
		return fmt.Errorf("invalid daily_report_time format %q: expected HH:MM (00:00–23:59)", value)
	}

	var settings models.AISettings
	dbErr := database.DB.Where("key = ?", dailyReportTimeKey).First(&settings).Error
	if dbErr == nil {
		settings.Value = value
		return database.DB.Save(&settings).Error
	}
	if !errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return dbErr
	}

	return database.DB.Create(&models.AISettings{
		Key:         dailyReportTimeKey,
		Value:       value,
		Description: "日报生成时刻（HH:MM）",
	}).Error
}

// LoadBoardUpgradeSuggestTimeConfig loads the semantic_board_upgrade_suggest_time
// setting from ai_settings. Returns the HH:MM string, or default "06:30" when
// the key is missing or invalid (mirrors LoadDailyReportTimeConfig).
func LoadBoardUpgradeSuggestTimeConfig() (string, error) {
	var settings models.AISettings
	err := database.DB.Where("key = ?", boardUpgradeSuggestTimeKey).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultBoardUpgradeSuggestTime, nil
		}
		return "", err
	}

	value := strings.TrimSpace(settings.Value)
	if !hhmmPattern.MatchString(value) {
		logging.Warnf("Invalid semantic_board_upgrade_suggest_time value %q, falling back to default %s", value, defaultBoardUpgradeSuggestTime)
		return defaultBoardUpgradeSuggestTime, nil
	}
	return value, nil
}

// SaveBoardUpgradeSuggestTimeConfig saves the semantic_board_upgrade_suggest_time
// setting. Validates HH:MM format (00:00–23:59). Returns error for invalid values.
func SaveBoardUpgradeSuggestTimeConfig(value string) error {
	value = strings.TrimSpace(value)
	if !hhmmPattern.MatchString(value) {
		return fmt.Errorf("invalid semantic_board_upgrade_suggest_time format %q: expected HH:MM (00:00–23:59)", value)
	}

	var settings models.AISettings
	dbErr := database.DB.Where("key = ?", boardUpgradeSuggestTimeKey).First(&settings).Error
	if dbErr == nil {
		settings.Value = value
		return database.DB.Save(&settings).Error
	}
	if !errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return dbErr
	}

	return database.DB.Create(&models.AISettings{
		Key:         boardUpgradeSuggestTimeKey,
		Value:       value,
		Description: "版块升级建议生成时刻（HH:MM）",
	}).Error
}
