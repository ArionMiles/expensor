package store

import (
	"reflect"
	"testing"
)

func TestReadModelRepositoryDoesNotKeepLegacyStoreBackReference(t *testing.T) {
	repoType := reflect.TypeOf(pgReadModelRepository{})
	if _, ok := repoType.FieldByName("legacy"); ok {
		t.Fatal("pgReadModelRepository should receive explicit dependencies instead of a legacy *Store back-reference")
	}
}
