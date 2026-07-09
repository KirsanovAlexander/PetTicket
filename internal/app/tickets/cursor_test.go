package tickets

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	createdAt := time.UnixMilli(1735689600123)
	id := int64(42)

	cursor := EncodeCursor(createdAt, id)
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	decodedTime, decodedID, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if decodedID != id {
		t.Errorf("expected id %d, got %d", id, decodedID)
	}
	if !decodedTime.Equal(createdAt) {
		t.Errorf("expected time %v, got %v", createdAt, decodedTime)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, _, err := DecodeCursor("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeCursor_InvalidFormat(t *testing.T) {
	// валидный base64, но без разделителя ":"
	cursor := "aGVsbG93b3JsZA==" // "helloworld"
	_, _, err := DecodeCursor(cursor)
	if err == nil {
		t.Fatal("expected error for cursor without separator")
	}
}

func TestDecodeCursor_InvalidTimestamp(t *testing.T) {
	cursor := base64.StdEncoding.EncodeToString([]byte("abc:1"))
	_, _, err := DecodeCursor(cursor)
	if err == nil {
		t.Fatal("expected error for non-numeric timestamp")
	}
}

func TestDecodeCursor_InvalidID(t *testing.T) {
	cursor := base64.StdEncoding.EncodeToString([]byte("1735689600123:abc"))
	_, _, err := DecodeCursor(cursor)
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestEncodeCursor_DifferentInputsProduceDifferentCursors(t *testing.T) {
	base := time.UnixMilli(1735689600000)
	c1 := EncodeCursor(base, 1)
	c2 := EncodeCursor(base, 2)
	if c1 == c2 {
		t.Error("expected different cursors for different ids")
	}
}
