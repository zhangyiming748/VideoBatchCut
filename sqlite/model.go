package sqlite

import (
	"errors"
	"log"
	"time"

	"gorm.io/gorm"
)

type BatchCut struct {
	Id       int64          `gorm:"primaryKey;autoIncrement;comment:主键"`
	Index    string         `gorm:"comment:索引"`
	Total    string         `gorm:"comment:总章节数"`
	FileName string         `gorm:"comment:文件名"`
	Start    string         `gorm:"comment:开始时间"`
	End      string         `gorm:"comment:结束时间"`
	CreateAt time.Time      `gorm:"autoCreateTime;comment:创建时间"`
	UpdateAt time.Time      `gorm:"autoUpdateTime;comment:更新时间"`
	DeleteAt gorm.DeletedAt `gorm:"index;comment:删除时间"`
}

func (b *BatchCut) Sync() {
	log.Printf("开始同步表结构\n")
	if err := GetSqlite().AutoMigrate(&BatchCut{}); err != nil {
		log.Fatalf("同步表结构Save失败:%s", err.Error())
	}
	log.Printf("同步表结构完成\n")
}

func (b *BatchCut) Insert() error {
	db := GetSqlite()
	if db == nil {
		return errors.New("数据库连接未初始化")
	}
	result := db.Create(&b)
	return result.Error
}

func (b *BatchCut) Update() error {
	db := GetSqlite()
	if db == nil {
		return errors.New("数据库连接未初始化")
	}
	result := db.Save(&b)
	return result.Error
}

func (b *BatchCut) Delete() error {
	db := GetSqlite()
	if db == nil {
		return errors.New("数据库连接未初始化")
	}
	result := db.Delete(&b)
	return result.Error
}

func (b *BatchCut) GetById(id int64) error {
	db := GetSqlite()
	if db == nil {
		return errors.New("数据库连接未初始化")
	}
	result := db.First(&b, id)
	return result.Error
}

func (b *BatchCut) GetAll() ([]BatchCut, error) {
	db := GetSqlite()
	if db == nil {
		return nil, errors.New("数据库连接未初始化")
	}
	var BatchCuts []BatchCut
	result := db.Find(&BatchCuts)
	return BatchCuts, result.Error
}
