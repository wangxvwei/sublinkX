package models

import (
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"
)

type SubLogs struct {
	ID            int
	IP            string
	Date          string
	Addr          string
	Count         int
	SubcriptionID int
	Country       string
	Province      string
	City          string
	ISP           string
	ASN           string
	Client        string
	UserAgent     string
	IPInfoSource  string
	IPInfoStatus  string
	IPInfoAt      *time.Time
}

// ponytail: one process writes this SQLite database; use a unique key/upsert if multi-instance deployment is added.
var accessLogMu sync.Mutex

func RecordSubscriptionAccess(id int, ip, client, userAgent string) (SubLogs, error) {
	accessLogMu.Lock()
	defer accessLogMu.Unlock()
	entry := SubLogs{IP: ip, SubcriptionID: id}
	err := DB.Where("ip = ? AND subcription_id = ?", ip, id).First(&entry).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return entry, err
	}
	entry.Count++
	entry.Date = time.Now().Format("2006-01-02 15:04:05")
	entry.Client, entry.UserAgent = client, userAgent
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = DB.Create(&entry).Error
	} else {
		err = DB.Model(&SubLogs{}).Where("id = ?", entry.ID).Updates(map[string]interface{}{
			"count": entry.Count, "date": entry.Date, "client": client, "user_agent": userAgent,
		}).Error
	}
	return entry, err
}

// Add 添加IP
func (iplog *SubLogs) Add() error {
	return DB.Create(iplog).Error
}

// 查找IP
func (iplog *SubLogs) Find(id int) error {
	return DB.Where("ip = ? and subcription_id  = ?", iplog.IP, id).First(iplog).Error
}

// Update 更新IP
func (iplog *SubLogs) Update() error {
	return DB.Where("id = ? AND subcription_id = ?", iplog.ID, iplog.SubcriptionID).Updates(iplog).Error
}

// List 获取IP列表
func (iplog *SubLogs) List() ([]SubLogs, error) {
	var iplogs []SubLogs
	err := DB.Find(&iplogs).Error
	if err != nil {
		return nil, err
	}
	return iplogs, nil
}

// Del 删除IP
func (iplog *SubLogs) Del() error {
	return DB.Delete(iplog).Error
}
