package assetrefs

// A test-only bridge lets the external public-client regression reuse the same
// bounded synthetic receipt fixture without a copied producer/validator.
import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"testing"
)

func NewVendorMetadataFixtureForExternalTest(t *testing.T) (Options, core.AssetReferenceQuery, func()) {
	f := vendorAssemblyGUIDFixture(t)
	mutate := func() {
		writeFile(t, f.opts.Root, f.opts.Project+"/Assets/ThirdParty/VendorShim.asmdef.meta", []byte("fileFormatVersion: 2\nguid: 66666666666666666666666666666666\n"))
	}
	return f.opts, core.AssetReferenceQuery{Object: objectID(meshGUID, i64(meshID)), Direction: "inbound"}, mutate
}
