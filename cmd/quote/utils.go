package main

import (
	"database/sql"
	"time"
)

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{Valid: true, String: s}
}

func nullTime(t time.Time) sql.NullTime {
	if t.Equal(time.Time{}) {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Valid: true, Time: t}
}

func nullInt32(i int) sql.NullInt32 {
	if i == 0 {
		return sql.NullInt32{Valid: false}
	}
	return sql.NullInt32{Valid: true, Int32: int32(i)}
}
