package sync

func NormalizeChangeKey(key string) string {
	switch key {
	case "users", "change_users":
		return "users"
	case "spaces", "change_spaces":
		return "spaces"
	case "space_members", "change_space_members", "change_space_menbers":
		return "space_members"
	case "photos", "change_photos":
		return "photos"
	case "expenses", "change_expenses":
		return "expenses"
	case "posts", "change_posts":
		return "posts"
	case "comments", "change_comments":
		return "comments"
	default:
		return ""
	}
}

func mergeBuckets(dst SyncChangeBucket, src SyncChangeBucket) SyncChangeBucket {
	dst.Created = append(dst.Created, src.Created...)
	dst.Updated = append(dst.Updated, src.Updated...)
	dst.Deleted = append(dst.Deleted, src.Deleted...)
	return dst
}

func NormalizeChangeMap(changes map[string]SyncChangeBucket) map[string]SyncChangeBucket {
	out := map[string]SyncChangeBucket{}
	for rawKey, bucket := range changes {
		key := NormalizeChangeKey(rawKey)
		if key == "" {
			continue
		}
		out[key] = mergeBuckets(out[key], bucket)
	}
	return out
}
