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

func TestStoreKeepsSingleImplementationRepositoriesConcrete(t *testing.T) {
	storeType := reflect.TypeOf(Store{})
	wantConcreteFields := []string{"community", "diag", "readModel", "rules", "runtime", "taxonomy", "txns"}
	for _, name := range wantConcreteFields {
		field, ok := storeType.FieldByName(name)
		if !ok {
			t.Fatalf("Store missing field %q", name)
		}
		if field.Type.Kind() == reflect.Interface {
			t.Fatalf("Store.%s is %s; single-implementation repositories should be concrete private fields", name, field.Type)
		}
	}
}
