package catalog_test

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/internal/catalogtest"
)

func TestTaskSelectionEvaluation(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	catalogtest.Run(t, c, func(t *testing.T, q catalog.Query) []byte {
		t.Helper()
		page, err := c.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		return body
	})
}
