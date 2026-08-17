package repo

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// toTime converts a non-null pgtype.Timestamptz to time.Time.
func toTime(ts pgtype.Timestamptz) time.Time {
	return ts.Time
}

// toTimePtr converts a nullable pgtype.Timestamptz to *time.Time.
func toTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// toStrPtr converts a nullable pgtype.Text to *string.
func toStrPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// toIntPtr converts a nullable pgtype.Int4 to *int.
func toIntPtr(i pgtype.Int4) *int {
	if !i.Valid {
		return nil
	}
	v := int(i.Int32)
	return &v
}

// fromIntPtr converts a *int to pgtype.Int4.
func fromIntPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// fromTimePtr converts a *time.Time to pgtype.Timestamptz.
func fromTimePtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// fromTime converts time.Time to pgtype.Timestamptz.
func fromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// fromStrPtr converts a *string to pgtype.Text.
func fromStrPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// fromStr converts a string to pgtype.Text.
func fromStr(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// fromUUIDPtr converts a *string UUID to pgtype.UUID. A nil value yields an
// invalid (NULL) pgtype.UUID.
func fromUUIDPtr(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	u, err := uuid.Parse(*s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

// uuidToString formats a pgtype.UUID as its canonical string form.
func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}
