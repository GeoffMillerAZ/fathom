# manifest.psd1 - a module manifest is a data file: a hashtable, no definitions.
@{
    RootModule        = 'module.psm1'
    ModuleVersion     = '1.0.0'
    FunctionsToExport = @('Get-CachedValue')
    PrivateData       = @{
        PSData = @{ Tags = @('demo') }
    }
}
