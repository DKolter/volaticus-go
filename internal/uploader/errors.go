package uploader

import "fmt"

var (
	ErrDuplicateURLValue = fmt.Errorf("duplicate URL value")
	ErrNoRows            = fmt.Errorf("no rows found")
	ErrTransaction       = fmt.Errorf("transaction error")
	ErrCommit            = fmt.Errorf("commit transaction error")
	ErrRollback          = fmt.Errorf("rollback transaction error")
)
