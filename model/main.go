package model

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"one-api/common"
	"one-api/common/config"
	"one-api/common/helper"
	"one-api/common/logger"
	"os"
	"strings"
	"time"
)

var DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != util.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		logger.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: helper.GetUUID(),
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func chooseDB() (*gorm.DB, error) {
	if os.Getenv("SQL_DSN") != "" {
		dsn := os.Getenv("SQL_DSN")
		if strings.HasPrefix(dsn, "postgres://") {
			// Use PostgreSQL
			logger.SysLog("using PostgreSQL as database")
			common.UsingPostgreSQL = true
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		logger.SysLog("using MySQL as database")
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	logger.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	config := fmt.Sprintf("?_busy_timeout=%d", common.SQLiteBusyTimeout)
	return gorm.Open(sqlite.Open(common.SQLitePath+config), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB()
	if err == nil {
		if config.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(helper.GetOrDefaultEnvInt("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(helper.GetOrDefaultEnvInt("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(helper.GetOrDefaultEnvInt("SQL_MAX_LIFETIME", 60)))

		if !config.IsMasterNode {
			return nil
		}
		logger.SysLog("database migration started")
		err = db.AutoMigrate(&Channel{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Token{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&User{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Option{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Redemption{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Ability{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Log{})
		if err != nil {
			return err
		}
		logger.SysLog("database migrated")
		err = createRootAccountIfNeed()
		return err
	} else {
		logger.FatalLog(err)
	}
	return err
}

func CloseDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}
