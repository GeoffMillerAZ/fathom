targetScope = 'resourceGroup'

param location string
param vaultId string
param tier string

resource account 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: 'example${uniqueString(resourceGroup().id)}'
  location: location
  sku: {
    name: tier
  }
  kind: 'StorageV2'
  properties: {
    encryption: {
      keySource: empty(vaultId) ? 'Microsoft.Storage' : 'Microsoft.Keyvault'
    }
  }
}

output accountName string = account.name
