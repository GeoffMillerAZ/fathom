package gallery

import (
	"net/http/httptest"
	"testing"
)

// The signed specimen's existing browser budget applies to the complete page,
// including both native navigation representations, not only its chart.
func TestSignedAnalyticsFullDocumentPayloadBudget(t *testing.T) {
	handler := New()
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", analyticsSignedPath+"?theme="+theme, nil))
			if response.Code != 200 {
				t.Fatalf("status %d", response.Code)
			}
			if n := response.Body.Len(); n >= 20*1024 {
				t.Errorf("full Gallery HTML = %d bytes, must remain under 20KiB", n)
			}
		})
	}
}
