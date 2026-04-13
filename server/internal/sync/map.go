package sync

import (
	"travel/internal/db"
)

func mapUsersForPull(rows []db.User) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":       r.ID,
			"nickname": r.Nickname,
		})
	}
	return out
}

func mapSpacesForPull(rows []db.Space) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":   r.ID,
			"name": r.Name,
		})
	}
	return out
}

func mapSpaceMembersForPull(rows []db.SpaceMember) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":       r.SpaceID + "_" + r.UserID,
			"space_id": r.SpaceID,
			"user_id":  r.UserID,
		})
	}
	return out
}

func mapPhotosForPull(rows []db.Photo) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":          r.ID,
			"space_id":    r.SpaceID,
			"uploader_id": r.UploaderID,
			"remote_url":  r.RemoteURL,
			"post_id":     r.PostID,
			"shoted_at":   NormalizeTSMillis(r.ShotedAt),
			"created_at":  NormalizeTSMillis(r.CreatedAt),
			"updated_at":  NormalizeTSMillis(r.UpdatedAt),
		})
	}
	return out
}

func mapExpensesForPull(rows []db.Expense) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":          r.ID,
			"space_id":    r.SpaceID,
			"payer_id":    r.PayerID,
			"amount":      r.Amount,
			"description": r.Description,
			"created_at":  NormalizeTSMillis(r.CreatedAt),
			"updated_at":  NormalizeTSMillis(r.UpdatedAt),
		})
	}
	return out
}

func mapPostsForPull(rows []db.Post) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":         r.ID,
			"space_id":   r.SpaceID,
			"created_at": NormalizeTSMillis(r.CreatedAt),
			"updated_at": NormalizeTSMillis(r.UpdatedAt),
		})
	}
	return out
}

func mapCommentsForPull(rows []db.Comment) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":           r.ID,
			"space_id":     r.SpaceID,
			"content":      r.Content,
			"commenter_id": r.CommenterID,
			"post_id":      r.PostID,
			"commented_at": NormalizeTSMillis(r.CommentedAt),
			"created_at":   NormalizeTSMillis(r.CreatedAt),
			"updated_at":   NormalizeTSMillis(r.UpdatedAt),
		})
	}
	return out
}
