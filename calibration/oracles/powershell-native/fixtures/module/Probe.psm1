function NativeOne { 1 }
function NativeTwo { 2 }
function PrivateTarget { 3 }
Set-Alias nativeAlias NativeOne
function PublicEntry { nativeAlias; PrivateTarget }
Export-ModuleMember -Function PublicEntry,NativeOne,NativeTwo -Alias nativeAlias
