package gallery

// Only reviewed first-party metadata belongs here. Prepared HTML/font bodies
// remain private. This receipt binds an explicitly chosen rendering environment.
pricingReference: #PricingDescriptor & {
	"receipt_sha256": "62a91c913f3860527b9d77b0b2227595d21b35ae1d06fb37a03a55091a30c8b8"
	"receipt_size":   2711
	"receipt": {
		"schema_version": 1
		"case_id":        "P02-pricing"
		"source": {
			"archive_id": "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
			"path":       "html/sections/pricing/05-single-price-with-details.html"
			"sha256":     "f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f"
			"size":       4710
		}
		"attempt_id":      "syntax@902c0695ff832fde0c73bb979cf73aa496098778061dffcff0276c11a6176043"
		"artifact_sha256": "3b45ede4d5b7652fa2c5d42c50b992fd3a743f958976fa9c014fec3497b6d23e"
		"transformer": {
			"id":             "html-to-templ-v1"
			"sha256":         "0db161e2e0e567ae491661614e0115f4042c3ea76b188f867c8242e44e90eb7c"
			"go_version":     "go1.26.3"
			"html_parser":    "golang.org/x/net@v0.54.0"
			"required_templ": "github.com/a-h/templ@v0.3.1020"
		}
		"configuration": {
			"tailwind_version":  "v4.3.0"
			"tailwind_platform": "darwin-arm64"
			"tailwind_sha256":   "56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e"
			"font": {
				"sha256": "693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3"
				"size":   352240
			}
			"font_family":          "InterVariable"
			"font_manifest_sha256": "dc7298020ee8a2e2502e7e038e8941c98123a03db4ffc8f52b6ff7fdc41f4655"
			"font_contract":        "unknown: explicit fixture font, not a claim about the original portal font"
			"css_input_sha256":     "719a3519c787bff8bb8b26fcedf9c190d814891c778ad0fcaeb4423245ed9a08"
			"css_output_sha256":    "3e3ee1b6faf2d1f97a6a30e072eb4f5cfce0c0771fba567a1438f60982c5740d"
			"theme_policy":         "explicit html.dark class; light body #ffffff; dark body #111827; vendor classes unchanged; no kit token mapping"
			"max_document_bytes":   1048576
		}
		"documents": {
			"converted-dark.html": {
				"sha256":       "544ed6ebb4bc71e4f0b0cc1966a4e4eb8fb7453ddfea131612d62e9e27f4c344"
				"size":         493544
				"style_sha256": "2fc828ba52cf9d931f612106374a007151f60860b610042f66886738bb520200"
			}
			"converted-light.html": {
				"sha256":       "ba9c18b67e564ab335b3a72a51b77779ec76eb1432c191c8f571e14560ff9872"
				"size":         493545
				"style_sha256": "2fc828ba52cf9d931f612106374a007151f60860b610042f66886738bb520200"
			}
			"original-dark.html": {
				"sha256":       "6b0352450e4f1672e6e42e64e8c0471a0461430bf9dcf4bf3e89cb0e5c8f148a"
				"size":         493524
				"style_sha256": "2fc828ba52cf9d931f612106374a007151f60860b610042f66886738bb520200"
			}
			"original-light.html": {
				"sha256":       "5d3a5473878298809b4e3c6d0a142e8379c56ffc974ee1a7dcadce0614ca04ad"
				"size":         493525
				"style_sha256": "2fc828ba52cf9d931f612106374a007151f60860b610042f66886738bb520200"
			}
		}
		"verification": "not-verified"
		"scope":        "controlled-static-reference"
	}
}

#SHA256: string & =~"^[a-f0-9]{64}$"
#PreparedDocument: {
	sha256:       #SHA256
	size:         int & >=1 & <=1048576
	style_sha256: #SHA256
}
#PricingDescriptor: {
	receipt_sha256: #SHA256
	receipt_size:   int & >=1 & <=65536
	receipt: {
		schema_version:  1
		case_id:         "P02-pricing"
		scope:           "controlled-static-reference"
		verification:    "not-verified"
		attempt_id:      "syntax@902c0695ff832fde0c73bb979cf73aa496098778061dffcff0276c11a6176043"
		artifact_sha256: #SHA256
		source: {
			archive_id: "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
			path:       "html/sections/pricing/05-single-price-with-details.html"
			sha256:     "f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f"
			size:       int & >=1 & <=262144
		}
		transformer: {
			id:             "html-to-templ-v1"
			sha256:         #SHA256
			go_version:     string
			html_parser:    "golang.org/x/net@v0.54.0"
			required_templ: "github.com/a-h/templ@v0.3.1020"
		}
		configuration: {
			tailwind_version:  "v4.3.0"
			tailwind_platform: "darwin-arm64"
			tailwind_sha256:   #SHA256
			font: {sha256: #SHA256, size: int & >=1 & <=524288}
			font_family:          "InterVariable"
			font_manifest_sha256: #SHA256
			font_contract:        string & !=""
			css_input_sha256:     #SHA256
			css_output_sha256:    #SHA256
			theme_policy:         string & !=""
			max_document_bytes:   1048576
		}
		documents: {
			"original-light.html":  #PreparedDocument
			"converted-light.html": #PreparedDocument
			"original-dark.html":   #PreparedDocument
			"converted-dark.html":  #PreparedDocument
		}
	}
}
