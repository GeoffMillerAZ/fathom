package harnessctl

import "strings"

#MaintenanceReceiverBinding: close({version!: "harnessctl.receiver-binding/1", controllerSHA256!: _hash, controllerEpoch!: _maintenanceEpoch, registrationSHA256!: _hash, registrationEpoch!: _maintenanceEpoch, hostId!: _id, projectId!: _id, principalId!: _id, stateRoot!: _abs})
#MaintenanceStatePointer: close({version!: "harnessctl.state-pointer/1", slot!: "a" | "b", sequence!: _maintenanceUint64 & >=1, sha256!: _hash})
#MaintenanceStateIntent: close({version!: "harnessctl.state-intent/1", from?: #MaintenanceStatePointer, to!: #MaintenanceStatePointer, operation!: "init" | "prepare" | "begin-approval" | "finish-approval" | "enter" | "check" | "end" | "lock" | "expire"})
#MaintenanceRecoveryEvidence: close({version!: "harnessctl.recovery-evidence/1", replacementState!: string & strings.MaxRunes(87384), bindingSHA256!: _hash, current!: string & strings.MaxRunes(5464), pending!: string & strings.MaxRunes(10924), stateA!: string & strings.MaxRunes(87384), stateB!: string & strings.MaxRunes(87384), currentTemp!: string & strings.MaxRunes(5464), pendingTemp!: string & strings.MaxRunes(10924), stateATempSHA256!: "" | _hash, stateBTempSHA256!: "" | _hash})
