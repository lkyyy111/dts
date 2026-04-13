package sync

import "time"

func NowMillis() int64 {
	return time.Now().UnixMilli()
}

func NormalizeTSMillis(ts int64) int64 {
	return ts
}
