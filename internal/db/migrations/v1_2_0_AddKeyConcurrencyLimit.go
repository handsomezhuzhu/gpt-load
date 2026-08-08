package db

import (
	"fmt"
	"strings"

	"gpt-load/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_2_0_AddKeyConcurrencyLimit adds the concurrency_limit column to the api_keys table.
// concurrency_limit = 0 means smart strategy (no hard cap, rely on 429 auto-switch),
// > 0 means a hard in-flight cap with proactive routing to other keys.
func V1_2_0_AddKeyConcurrencyLimit(db *gorm.DB) error {
	if db.Migrator().HasColumn(&models.APIKey{}, "concurrency_limit") {
		logrus.Info("Column concurrency_limit already exists, skipping v1.2.0...")
		return nil
	}

	if err := db.Migrator().AddColumn(&models.APIKey{}, "concurrency_limit"); err != nil {
		if strings.Contains(err.Error(), "duplicate column") {
			logrus.Info("Column concurrency_limit already exists, skipping v1.2.0...")
			return nil
		}
		return fmt.Errorf("failed to add concurrency_limit column to api_keys: %w", err)
	}

	logrus.Info("Migration v1.2.0 completed successfully")
	return nil
}
