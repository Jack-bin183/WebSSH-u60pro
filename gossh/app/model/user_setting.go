package model

type UserSetting struct {
	ID    uint   `gorm:"column:id;primaryKey,autoIncrement" json:"id"`
	Uid   uint   `gorm:"column:uid;not null;uniqueIndex" json:"uid"`
	Name  string `gorm:"column:name;not null;size:64;default:'default'" json:"name"`
	Value string `gorm:"column:value;type:text" json:"value"`

	CreatedAt DateTime `gorm:"column:created_at" json:"-"`
	UpdatedAt DateTime `gorm:"column:updated_at" json:"-"`
}

func (s UserSetting) FindByUid(uid uint) (UserSetting, error) {
	var setting UserSetting
	err := Db.First(&setting, "uid = ?", uid).Error
	return setting, err
}

func (s UserSetting) FindLatestNonEmpty() (UserSetting, error) {
	var setting UserSetting
	err := Db.Where("value <> ?", "").Order("updated_at desc").First(&setting).Error
	return setting, err
}

func (s UserSetting) SaveForUid(uid uint, value string) error {
	var setting UserSetting
	err := Db.First(&setting, "uid = ?", uid).Error
	if err == nil {
		setting.Value = value
		return Db.Model(&setting).Where("uid = ?", uid).Updates(setting).Error
	}

	setting = UserSetting{
		Uid:   uid,
		Name:  "network_wifi",
		Value: value,
	}
	return Db.Create(&setting).Error
}
