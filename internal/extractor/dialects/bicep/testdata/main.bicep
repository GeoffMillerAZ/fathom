metadata description = 'An example deployment'
import { skuName as sku, tagSet } from './types.bicep'
import * as shared from './types.bicep'

@description('Where every resource goes')
param location string = resourceGroup().location

@allowed([
  'dev'
  'prod'
])
param env string = 'dev'
param tier sku
param names array = [
  'a'
  'b'
]

type settings = {
  tags: tagSet
  retention: int
}

func prefixed(value string) string => 'ex-${value}'

var vnetName = prefixed('vnet')
var subnets = [for (n, i) in names: {
  name: n
  prefix: '10.0.${i}.0/24'
}]
var upper = map(names, x => toUpper(x))
var isProd = env == 'prod' ? true : false

resource vault 'Microsoft.KeyVault/vaults@2023-07-01' existing = {
  name: 'example-kv'
}

resource vnet 'Microsoft.Network/virtualNetworks@2023-09-01' = {
  name: vnetName
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [
        '10.0.0.0/16'
      ]
    }
  }

  resource subnet 'subnets' = {
    name: 'default'
    properties: {
      addressPrefix: subnets[0].prefix
    }
  }
}

resource nics 'Microsoft.Network/networkInterfaces@2023-09-01' = [for n in names: if (isProd) {
  name: 'nic-${n}'
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          subnet: {
            id: vnet::subnet.id
          }
        }
      }
    ]
  }
}]

module storage './storage.bicep' = {
  name: 'storage'
  params: {
    location: location
    vaultId: vault.id
    tier: tier
  }
}

module dns 'br/public:avm/res/network/dns-zone:0.5.0' = {
  name: 'dns'
  params: {
    name: 'example.com'
  }
}

output vnetId string = vnet.id
output accountName string = storage.outputs.accountName
output nicNames array = [for n in names: 'nic-${n}']
