package handler

import (
	"fmt"
	"time"
)

// PG は timestamp を time.Time で返すが、テストで使う sqlite TEXT カラムは string で返る。
// 両方を time.Time として受けるため sql.Scanner を実装する。
type scanTime struct{ time.Time }

func (t *scanTime) Scan(src any) error {
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
	return fmt.Errorf("scanTime: unsupported type %T", src)
}

func (t *scanTime) parse(s string) error {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("scanTime: cannot parse %q", s)
}
