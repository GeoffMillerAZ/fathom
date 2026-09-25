package gallery

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/geoffmilleraz/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

// Only successful evidence HTML opts into same-origin images. Other private
// pages and all failures keep the registrar's deny-by-default policy.
const evidenceImagesCSP = "default-src 'none'; style-src 'self'; img-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// Native browser image documents generate their own inline presentation. This
// exception applies only after exact PNG validation, never to HTML or errors.
const evidencePNGDocumentCSP = "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

var evidenceImageID = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,95}$`)

func parseEvidenceDetailRequest(values url.Values, api bool) (evidenceRequest, error) {
	if api {
		return parseEvidenceRequest(values, true)
	}
	base := maps.Clone(values)
	for _, key := range []string{"image", "compare"} {
		items := values[key]
		if len(items) > 1 || (len(items) == 1 && items[0] != "" && !evidenceImageID.MatchString(items[0])) {
			return evidenceRequest{}, errors.New("invalid image selection")
		}
		base.Del(key)
	}
	in, err := parseEvidenceRequest(base, false)
	if err != nil {
		return in, err
	}
	for _, key := range []string{"image", "compare"} {
		if values.Get(key) != "" {
			in.Values.Set(key, values.Get(key))
		}
	}
	return in, nil
}

type evidenceImageView struct {
	Evidence designreceipts.Evidence
	URL      string
	Width    int
	Height   int
	Problem  string
}

type evidenceImagesView struct {
	Choices        []designreceipts.Evidence
	Selected       []evidenceImageView
	Image, Compare string
	Action         string
	Context        url.Values
}

func buildEvidenceImages(root string, detail designevidence.Detail, values url.Values) (evidenceImagesView, error) {
	v := evidenceImagesView{Action: "/target-evidence/" + detail.Entry.ID, Context: maps.Clone(values), Image: values.Get("image"), Compare: values.Get("compare")}
	v.Context.Del("image")
	v.Context.Del("compare")
	seen := map[string]bool{}
	byID := map[string]designreceipts.Evidence{}
	for _, result := range detail.Receipt.Results {
		for _, item := range result.Evidence {
			if item.Role == "image" && !seen[item.ID] {
				v.Choices = append(v.Choices, item)
				seen[item.ID] = true
				byID[item.ID] = item
			}
		}
	}
	sort.Slice(v.Choices, func(i, j int) bool { return v.Choices[i].ID < v.Choices[j].ID })
	if v.Image == "" && len(v.Choices) > 0 {
		v.Image = v.Choices[0].ID
	}
	for _, id := range []string{v.Image, v.Compare} {
		if id == "" {
			continue
		}
		if !seen[id] {
			return v, errors.New("capture is not declared image evidence")
		}
		capture, err := designevidence.ReadImage(root, detail.Entry.ID, id)
		if err != nil {
			v.Selected = append(v.Selected, evidenceImageView{Evidence: byID[id], Problem: "This capture could not pass retained-byte and PNG checks. It has not been displayed; metadata and original recovery remain available."})
			continue
		}
		v.Selected = append(v.Selected, evidenceImageView{Evidence: capture.Evidence, URL: v.Action + "/images/" + id, Width: capture.Width, Height: capture.Height})
	}
	return v, nil
}

func registerEvidenceImages(root string, register func(string, bool, http.HandlerFunc)) {
	register("/target-evidence/{id}/images/{slot}", true, func(w http.ResponseWriter, r *http.Request) {
		id, slot := r.PathValue("id"), r.PathValue("slot")
		if _, err := parseEvidenceRequest(r.URL.Query(), true); err != nil || !evidenceImageID.MatchString(slot) || r.URL.EscapedPath() != "/target-evidence/"+id+"/images/"+slot {
			http.Error(w, "Invalid exact capture request", http.StatusBadRequest)
			return
		}
		if root == "" {
			http.Error(w, "Private library is not configured", http.StatusServiceUnavailable)
			return
		}
		capture, err := designevidence.ReadImage(root, id, slot)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, os.ErrNotExist) {
				status = http.StatusNotFound
			}
			http.Error(w, "Retained capture unavailable; no target compatibility is implied", status)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Security-Policy", evidencePNGDocumentCSP)
		w.Header().Set("Content-Length", strconv.Itoa(len(capture.PNG)))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s.png"`, capture.Evidence.ID))
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Appkit-Evidence-SHA256", capture.Evidence.SHA256)
		w.Header().Set("X-Appkit-Verification", "not-verified")
		_, _ = w.Write(capture.PNG)
	})
}
