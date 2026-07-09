package tickets

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultCursorPageSize используется, если клиент не указал page_size.
const DefaultCursorPageSize = 20

// MaxCursorPageSize — верхняя граница page_size (защита от чрезмерных выборок).
const MaxCursorPageSize = 100

// EncodeCursor кодирует пару (createdAt, id) в непрозрачный cursor-токен вида
// base64("unixMilli:id"). Пара (created_at, id) выбрана как ключ, потому что
// created_at сам по себе не уникален (несколько тикетов могут быть созданы
// в одну миллисекунду) — id обеспечивает полный порядок для устойчивой
// пагинации.
func EncodeCursor(createdAt time.Time, id int64) string {
	data := fmt.Sprintf("%d:%d", createdAt.UnixMilli(), id)
	return base64.StdEncoding.EncodeToString([]byte(data))
}

// DecodeCursor декодирует cursor обратно в (createdAt, id).
func DecodeCursor(cursor string) (time.Time, int64, error) {
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}

	millis, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor timestamp: %w", err)
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor id: %w", err)
	}

	return time.UnixMilli(millis), id, nil
}
