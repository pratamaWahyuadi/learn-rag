package repo

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// newUUIDString returns a fresh random UUID string.
func newUUIDString() string {
	return uuid.NewString()
}

// nowTimestamptz returns the current time as a valid pgtype.Timestamptz.
func nowTimestamptz() pgtype.Timestamptz {
	return fromTime(time.Now())
}
