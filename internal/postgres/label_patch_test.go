package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestUpdateLabelMetadata(t *testing.T) {
	db := newTestDB(t)
	insertTestLabels(t, db)

	primary := true
	label, err := UpdateLabelMetadata(t.Context(), db, "label-10", LabelMetadataPatch{IsPrimary: &primary})
	if err != nil {
		t.Fatal(err)
	}
	if !label.IsPrimary || label.Nickname == nil || *label.Nickname != "Desk" {
		t.Fatalf("expected primary flag set and nickname preserved, got %+v", label)
	}

	label, err = UpdateLabelMetadata(t.Context(), db, "label-1", LabelMetadataPatch{HasNickname: true})
	if err != nil {
		t.Fatal(err)
	}
	if label.Nickname != nil || !label.IsPrimary {
		t.Fatalf("expected cleared nickname and independent primary flag, got %+v", label)
	}

	secondary := false
	nickname := "Dispatch"
	patch := LabelMetadataPatch{HasNickname: true, Nickname: &nickname, IsPrimary: &secondary}
	label, err = UpdateLabelMetadata(t.Context(), db, "label-1", patch)
	if err != nil {
		t.Fatal(err)
	}
	if label.IsPrimary || label.Nickname == nil || *label.Nickname != nickname || label.Version != "3" {
		t.Fatalf("unexpected updated label: %+v", label)
	}

	for _, id := range []string{"label-11", "missing"} {
		t.Run(id, func(t *testing.T) {
			_, err := UpdateLabelMetadata(t.Context(), db, id, patch)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("expected ErrNoRows, got %v", err)
			}
		})
	}
}
