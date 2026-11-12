package db

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/migrations"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	if dsn := os.Getenv("SHOP_TEST_DSN"); dsn != "" {
		cfg.Mysql.DSN = dsn
	} else {
		cfg.Mysql.DSN = "root:root@tcp(127.0.0.1:3307)/shop_test_db?charset=utf8mb4&parseTime=True&loc=Local"
	}
	cfg.Mysql.MaxIdleConns = 2
	cfg.Mysql.MaxOpenConns = 4
	return cfg
}

func TestGormCreateDatabaseAndMigrate(t *testing.T) {
	cfg := testConfig(t)
	// 库不存在时自动创建（覆盖 createDatabase 分支）
	cfg.Mysql.DSN = "root:root@tcp(127.0.0.1:3307)/shop_test_db_autocreate?charset=utf8mb4&parseTime=True&loc=Local"
	gdb, err := Gorm(cfg)
	if err != nil {
		t.Fatalf("自动建库连接失败: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()

	// 空迁移集报错
	if err := Migrate(gdb, fstest.MapFS{}); err == nil {
		t.Fatal("空迁移集应报错")
	}
	// 非法 SQL 报错
	bad := fstest.MapFS{"001_bad.sql": &fstest.MapFile{Data: []byte("CREATE TABLE ;")}}
	if err := Migrate(gdb, bad); err == nil {
		t.Fatal("非法 SQL 应报错")
	}
	// 正常迁移与种子补齐
	cfg.Mysql.DSN = "root:root@tcp(127.0.0.1:3307)/shop_test_db?charset=utf8mb4&parseTime=True&loc=Local"
	gdb2, err := Gorm(cfg)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	if err := Migrate(gdb2, migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if err := SeedAccounts(gdb2, "shop123"); err != nil {
		t.Fatalf("种子账号失败: %v", err)
	}
	// 幂等：重复补齐不报错
	if err := SeedAccounts(gdb2, ""); err != nil {
		t.Fatalf("重复种子账号失败: %v", err)
	}
	var count int64
	if err := gdb2.Table("users").Where("password_hash <> ''").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("种子账号未补齐密码")
	}
}

func TestSplitSQL(t *testing.T) {
	stmts := splitSQL("-- 注释\nCREATE TABLE a (id INT);\n\nINSERT INTO a VALUES (1);\n")
	if len(stmts) != 2 {
		t.Fatalf("语句切分错误: %d %+v", len(stmts), stmts)
	}
}

// errFS 读取即失败，用于覆盖迁移读取错误分支。
type errFS struct{}

func (errFS) Open(string) (fs.File, error) { return nil, errors.New("read failure") }

func TestGormAndMigrateErrorBranches(t *testing.T) {
	// 非法 DSN：非「库不存在」类错误直接返回
	cfg := testConfig(t)
	cfg.Mysql.DSN = "not-a-valid-dsn"
	if _, err := Gorm(cfg); err == nil {
		t.Fatal("非法 DSN 应报错")
	}
	// DSN 未指定库名：自动建库内部必须报错（直接调内部函数，避免连接默认库成功）
	if err := createDatabase("root:root@tcp(127.0.0.1:3307)/?charset=utf8mb4"); err == nil {
		t.Fatal("未指定库名应报错")
	}

	// 迁移文件读取失败
	cfg = testConfig(t)
	gdb, err := Gorm(cfg)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	if err := Migrate(gdb, errFS{}); err == nil {
		t.Fatal("文件读取失败应报错")
	}

	// 连接关闭后种子补齐必须报错（不允许静默成功）
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()
	if err := SeedAccounts(gdb, "shop123"); err == nil {
		t.Fatal("连接关闭后应报错")
	}
}
