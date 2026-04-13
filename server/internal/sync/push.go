package sync

import (
	"errors"
	"fmt"
	"travel/internal/db"

	"gorm.io/gorm"
)

func ApplySyncChanges(tx *gorm.DB, changes map[string]SyncChangeBucket, lastPulledAt int64) error {
	type groupHandler struct {
		key     string
		handler func(*gorm.DB, SyncChangeBucket, int64) error
	}
	handlers := []groupHandler{
		{"users", applyUserChanges},
		{"spaces", applySpaceChanges},
		{"space_members", applySpaceMemberChanges},
		{"photos", applyPhotoChanges},
		{"expenses", applyExpenseChanges},
		{"posts", applyPostChanges},
		{"comments", applyCommentChanges},
	}
	for _, h := range handlers {
		bucket, ok := changes[h.key]
		if !ok {
			continue
		}
		if err := h.handler(tx, bucket, lastPulledAt); err != nil {
			return err
		}
	}
	return nil
}

func applyUserChanges(tx *gorm.DB, bucket SyncChangeBucket, _ int64) error {
	created, err := DecodeBucket[SyncUser](bucket.Created)
	if err != nil {
		return fmt.Errorf("process users created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncUser](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process users updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if err := upsertUser(tx, item); err != nil {
			return fmt.Errorf("process users upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if err := upsertUser(tx, item); err != nil {
			return fmt.Errorf("process users upsert failed: %w", err)
		}
	}
	return nil
}

func applySpaceChanges(tx *gorm.DB, bucket SyncChangeBucket, _ int64) error {
	created, err := DecodeBucket[SyncSpace](bucket.Created)
	if err != nil {
		return fmt.Errorf("process spaces created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncSpace](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process spaces updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if err := upsertSpace(tx, item); err != nil {
			return fmt.Errorf("process spaces upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if err := upsertSpace(tx, item); err != nil {
			return fmt.Errorf("process spaces upsert failed: %w", err)
		}
	}
	return nil
}

func applySpaceMemberChanges(tx *gorm.DB, bucket SyncChangeBucket, _ int64) error {
	created, err := DecodeBucket[SyncSpaceMember](bucket.Created)
	if err != nil {
		return fmt.Errorf("process space_members created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncSpaceMember](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process space_members updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		normalized, err := NormalizeSpaceMemberID(item)
		if err != nil {
			return fmt.Errorf("process space_members failed: %s: %w", err.Error(), gorm.ErrInvalidData)
		}
		if err := upsertSpaceMember(tx, normalized); err != nil {
			return fmt.Errorf("process space_members upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		normalized, err := NormalizeSpaceMemberID(item)
		if err != nil {
			return fmt.Errorf("process space_members failed: %s: %w", err.Error(), gorm.ErrInvalidData)
		}
		if err := upsertSpaceMember(tx, normalized); err != nil {
			return fmt.Errorf("process space_members upsert failed: %w", err)
		}
	}
	return nil
}

func applyPhotoChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := DecodeBucket[SyncPhoto](bucket.Created)
	if err != nil {
		return fmt.Errorf("process photos created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncPhoto](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process photos updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertPhoto(tx, item, PushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process photos upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertPhoto(tx, item, PushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process photos upsert failed: %w", err)
		}
	}
	if err := softDeleteContentByIDs(tx, "photos", bucket.Deleted, NowMillis()); err != nil {
		return fmt.Errorf("process photos deleted failed: %w", err)
	}
	return nil
}

func applyExpenseChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := DecodeBucket[SyncExpense](bucket.Created)
	if err != nil {
		return fmt.Errorf("process expenses created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncExpense](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process expenses updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertExpense(tx, item, PushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process expenses upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertExpense(tx, item, PushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process expenses upsert failed: %w", err)
		}
	}
	if err := softDeleteContentByIDs(tx, "expenses", bucket.Deleted, NowMillis()); err != nil {
		return fmt.Errorf("process expenses deleted failed: %w", err)
	}
	return nil
}

func applyPostChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := DecodeBucket[SyncPost](bucket.Created)
	if err != nil {
		return fmt.Errorf("process posts created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncPost](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process posts updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertPost(tx, item, PushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process posts upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertPost(tx, item, PushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process posts upsert failed: %w", err)
		}
	}
	if err := softDeleteContentByIDs(tx, "posts", bucket.Deleted, NowMillis()); err != nil {
		return fmt.Errorf("process posts deleted failed: %w", err)
	}
	return nil
}

func applyCommentChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := DecodeBucket[SyncComment](bucket.Created)
	if err != nil {
		return fmt.Errorf("process comments created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := DecodeBucket[SyncComment](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process comments updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertComment(tx, item, PushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process comments upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertComment(tx, item, PushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process comments upsert failed: %w", err)
		}
	}
	if err := softDeleteContentByIDs(tx, "comments", bucket.Deleted, NowMillis()); err != nil {
		return fmt.Errorf("process comments deleted failed: %w", err)
	}
	return nil
}

func upsertUser(tx *gorm.DB, item SyncUser) error {
	if item.ID == "" {
		return nil
	}
	if !IsValidULID(item.ID) {
		return fmt.Errorf("invalid ULID for users.id: %s: %w", item.ID, gorm.ErrInvalidData)
	}
	var existing db.User
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	record := db.User{
		ID:       item.ID,
		Nickname: item.Nickname,
	}
	switch {
	case err == nil:
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"nickname": record.Nickname,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		if record.Nickname == "" {
			record.Nickname = "user-" + item.ID
		}
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertSpace(tx *gorm.DB, item SyncSpace) error {
	if item.ID == "" {
		return nil
	}
	if !IsValidULID(item.ID) {
		return fmt.Errorf("invalid ULID for spaces.id: %s: %w", item.ID, gorm.ErrInvalidData)
	}
	var existing db.Space
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	record := db.Space{
		ID:   item.ID,
		Name: item.Name,
	}
	switch {
	case err == nil:
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"name": record.Name,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertSpaceMember(tx *gorm.DB, item SyncSpaceMember) error {
	var spaceCount, userCount int64
	if err := tx.Model(&db.Space{}).Where("id = ?", item.SpaceID).Count(&spaceCount).Error; err != nil {
		return err
	}
	if spaceCount == 0 {
		return fmt.Errorf("space_members: space_id not found: %w", gorm.ErrInvalidData)
	}
	if err := tx.Model(&db.User{}).Where("id = ?", item.UserID).Count(&userCount).Error; err != nil {
		return err
	}
	if userCount == 0 {
		return fmt.Errorf("space_members: user_id not found: %w", gorm.ErrInvalidData)
	}

	var existing db.SpaceMember
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	record := db.SpaceMember{
		ID:      item.ID,
		SpaceID: item.SpaceID,
		UserID:  item.UserID,
	}
	switch {
	case err == nil:
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id": record.SpaceID,
			"user_id":  record.UserID,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertPhoto(tx *gorm.DB, item SyncPhoto, mode PushMode, lastPulledAt int64) error {
	var existing db.Photo
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := NowMillis()
	record := db.Photo{
		ID:         item.ID,
		SpaceID:    item.SpaceID,
		UploaderID: item.UploaderID,
		RemoteURL:  item.RemoteURL,
		PostID:     item.PostID,
		ShotedAt:   NormalizeTSMillis(item.ShotedAt),
		CreatedAt:  NormalizeTSMillis(item.CreatedAt),
		UpdatedAt:  NormalizeTSMillis(item.UpdatedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == PushModeUpdated && existing.LastModified > lastPulledAt {
			return ConflictError{Table: "photos", ID: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":      record.SpaceID,
			"uploader_id":   record.UploaderID,
			"remote_url":    record.RemoteURL,
			"post_id":       record.PostID,
			"shoted_at":     record.ShotedAt,
			"created_at":    record.CreatedAt,
			"updated_at":    record.UpdatedAt,
			"deleted_at":    record.DeletedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreatedAt = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertExpense(tx *gorm.DB, item SyncExpense, mode PushMode, lastPulledAt int64) error {
	var existing db.Expense
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := NowMillis()
	record := db.Expense{
		ID:          item.ID,
		SpaceID:     item.SpaceID,
		PayerID:     item.PayerID,
		Amount:      item.Amount,
		Description: item.Description,
		CreatedAt:   NormalizeTSMillis(item.CreatedAt),
		UpdatedAt:   NormalizeTSMillis(item.UpdatedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == PushModeUpdated && existing.LastModified > lastPulledAt {
			return ConflictError{Table: "expenses", ID: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":      record.SpaceID,
			"payer_id":      record.PayerID,
			"amount":        record.Amount,
			"description":   record.Description,
			"created_at":    record.CreatedAt,
			"updated_at":    record.UpdatedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreatedAt = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertPost(tx *gorm.DB, item SyncPost, mode PushMode, lastPulledAt int64) error {
	var existing db.Post
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := NowMillis()
	record := db.Post{
		ID:        item.ID,
		SpaceID:   item.SpaceID,
		CreatedAt: NormalizeTSMillis(item.CreatedAt),
		UpdatedAt: NormalizeTSMillis(item.UpdatedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == PushModeUpdated && existing.LastModified > lastPulledAt {
			return ConflictError{Table: "posts", ID: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":      record.SpaceID,
			"created_at":    record.CreatedAt,
			"updated_at":    record.UpdatedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreatedAt = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertComment(tx *gorm.DB, item SyncComment, mode PushMode, lastPulledAt int64) error {
	var existing db.Comment
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := NowMillis()
	record := db.Comment{
		ID:          item.ID,
		SpaceID:     item.SpaceID,
		Content:     item.Content,
		CommenterID: item.CommenterID,
		PostID:      item.PostID,
		CommentedAt: NormalizeTSMillis(item.CommentedAt),
		CreatedAt:   NormalizeTSMillis(item.CreatedAt),
		UpdatedAt:   NormalizeTSMillis(item.UpdatedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == PushModeUpdated && existing.LastModified > lastPulledAt {
			return ConflictError{Table: "comments", ID: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":      record.SpaceID,
			"content":       record.Content,
			"commenter_id":  record.CommenterID,
			"post_id":       record.PostID,
			"commented_at":  record.CommentedAt,
			"created_at":    record.CreatedAt,
			"updated_at":    record.UpdatedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreatedAt = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func softDeleteContentByIDs(tx *gorm.DB, table string, ids []string, ts int64) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Table(table).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"deleted_at":    ts,
			"updated_at":    ts,
			"last_modified": ts,
		}).Error
}
