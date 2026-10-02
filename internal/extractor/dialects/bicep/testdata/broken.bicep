param good string = 'ok'

resource half 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: good
  location: )
}

var after = good

output last string = after
