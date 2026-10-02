@export()
type skuName = 'Standard_LRS' | 'Premium_LRS'

@export()
type tagSet = {
  *: string
}

@export()
func label(name string) string => 'example-${name}'
