module "example.com/quoted"

go 1.20

require (
	// a comment line inside the block
	"github.com/old/pkg" v2.5.0+incompatible
	golang.org/x/text v0.0.0-20210101000000-abcdef123456 // indirect

	gopkg.in/yaml.v3 v3.0.1
)
