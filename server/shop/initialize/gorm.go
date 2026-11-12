// Package initialize 负责 Shop 业务基础设施初始化：连接、迁移、种子账号。
package initialize

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/utils"
)

// Gorm 建立连接：库不存在时自动创建，保证首次运行可直接启动。
func Gorm(cfg config.Config) (*gorm.DB, error) {
	newLogger := logger.Default.LogMode(logger.Silent)
	if cfg.Mysql.LogMode {
		newLogger = logger.Default.LogMode(logger.Info)
	}
	db, err := gorm.Open(mysql.Open(cfg.Mysql.DSN), &gorm.Config{Logger: newLogger})
	if err != nil {
		if !strings.Contains(err.Error(), "Unknown database") {
			return nil, err
		}
		if cerr := createDatabase(cfg.Mysql.DSN); cerr != nil {
			return nil, cerr
		}
		db, err = gorm.Open(mysql.Open(cfg.Mysql.DSN), &gorm.Config{Logger: newLogger})
		if err != nil {
			return nil, err
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.Mysql.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Mysql.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func createDatabase(dsn string) error {
	parsed, err := driver.ParseDSN(dsn)
	if err != nil {
		return err
	}
	name := parsed.DBName
	if name == "" {
		return fmt.Errorf("DSN 未指定数据库名")
	}
	parsed.DBName = ""
	boot, err := gorm.Open(mysql.Open(parsed.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", name)
	if err := boot.Exec(stmt).Error; err != nil {
		return err
	}
	conn, err := boot.DB()
	if err == nil {
		_ = conn.Close()
	}
	return nil
}

// Migrate 按文件名顺序执行内置 migrations，脚本全部使用 IF NOT EXISTS 可重复执行。
func Migrate(db *gorm.DB, fsys fs.FS) error {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("内置迁移脚本为空")
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(raw)) {
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("%s 执行失败: %w", name, err)
			}
		}
	}
	return nil
}

// splitSQL 去掉注释行后按分号切分语句。
func splitSQL(content string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}
	var out []string
	for _, stmt := range strings.Split(cleaned.String(), ";") {
		if s := strings.TrimSpace(stmt); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// SeedAccounts 为预置账号补齐密码摘要（种子脚本不落明文，也不落固定哈希）。
func SeedAccounts(db *gorm.DB, demoPassword string) error {
	if demoPassword == "" {
		demoPassword = "shop123"
	}
	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		if strings.TrimSpace(u.PasswordHash) != "" {
			continue
		}
		hash, err := utils.HashPassword(demoPassword)
		if err != nil {
			return err
		}
		if err := db.Model(&model.User{}).Where("id = ?", u.ID).Update("password_hash", hash).Error; err != nil {
			return err
		}
	}
	return nil
}
