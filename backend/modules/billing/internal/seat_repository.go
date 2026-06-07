package billing

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type seatRepository interface {
	occupy(userID int, capacity int64) (bool, error) // 原子：used<capacity → +1，返回是否成功
	release(userID int) error
	reconcile(userID int, count int64) error
	usage(userID int) (int64, error)
	listUsages() ([]SeatUsage, error)
	getDunning(userID int) (*DunningState, error)
	upsertDunning(userID int, state string, enteredAt time.Time) error
	listDunningByStates(states []string) ([]DunningState, error)
}

type gormSeatRepository struct{ db *gorm.DB }

func newSeatRepository(db *gorm.DB) seatRepository { return &gormSeatRepository{db: db} }

// occupy 原子占用：保证行存在后，守卫式 UPDATE used=used+1 WHERE used < capacity。
func (r *gormSeatRepository) occupy(userID int, capacity int64) (bool, error) {
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&SeatUsage{UserID: uint(userID)}).Error; err != nil {
		return false, err
	}
	res := r.db.Model(&SeatUsage{}).
		Where("user_id = ? AND instance_seats_used < ?", userID, capacity).
		UpdateColumn("instance_seats_used", gorm.Expr("instance_seats_used + 1"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *gormSeatRepository) release(userID int) error {
	return r.db.Model(&SeatUsage{}).
		Where("user_id = ? AND instance_seats_used > 0", userID).
		UpdateColumn("instance_seats_used", gorm.Expr("instance_seats_used - 1")).Error
}

func (r *gormSeatRepository) reconcile(userID int, count int64) error {
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&SeatUsage{UserID: uint(userID)}).Error; err != nil {
		return err
	}
	return r.db.Model(&SeatUsage{}).Where("user_id = ?", userID).
		UpdateColumn("instance_seats_used", count).Error
}

func (r *gormSeatRepository) usage(userID int) (int64, error) {
	var u SeatUsage
	err := r.db.Where("user_id = ?", userID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return u.InstanceSeatsUsed, err
}

func (r *gormSeatRepository) listUsages() ([]SeatUsage, error) {
	var items []SeatUsage
	err := r.db.Find(&items).Error
	return items, err
}

func (r *gormSeatRepository) getDunning(userID int) (*DunningState, error) {
	var d DunningState
	err := r.db.Where("user_id = ?", userID).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &DunningState{UserID: uint(userID), State: DunningActive}, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *gormSeatRepository) upsertDunning(userID int, state string, enteredAt time.Time) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"state", "entered_at", "updated_at"}),
	}).Create(&DunningState{UserID: uint(userID), State: state, EnteredAt: enteredAt}).Error
}

func (r *gormSeatRepository) listDunningByStates(states []string) ([]DunningState, error) {
	var items []DunningState
	err := r.db.Where("state IN ?", states).Find(&items).Error
	return items, err
}
