package sync

import "fmt"

type ConflictError struct {
	Table string
	ID    string
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("conflict on %s: id=%s modified after last_pulled_at", e.Table, e.ID)
}
