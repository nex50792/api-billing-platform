package repository

import (
	"fmt"
	"time"
)

// flexTime は PG の timestamp (time.Time) とテスト用 sqlite の TEXT の両方を
// time.Time として受けるための sql.Scanner 実装。handler.scanTime と同じロジック。
type flexTime struct{ time.Time }

func (t *flexTime) Scan(src any) error {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		t.Time = v
		return nil
	case string:
		return t.parse(v)
	case []byte:
		return t.parse(string(v))
	}
	return fmt.Errorf("flexTime: unsupported type %T", src)
}

func (t *flexTime) parse(s string) error {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("flexTime: cannot parse %q", s)
}
