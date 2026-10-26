package main

type registration struct {
	Version   string    `json:"version"`
	Epoch     int       `json:"epoch"`
	IssuedAt  string    `json:"issuedAt"`
	ExpiresAt string    `json:"expiresAt"`
	HostID    string    `json:"hostId"`
	Keys      []key     `json:"keys"`
	Controls  []control `json:"controls"`
	Projects  []project `json:"projects"`
}
type key struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Revoked   bool   `json:"revoked"`
}
type project struct {
	ID             string     `json:"id"`
	Root           string     `json:"root"`
	OrganizationID *string    `json:"organizationId,omitempty"`
	TeamIDs        []string   `json:"teamIds"`
	PrincipalID    string     `json:"principalId"`
	ControlIDs     []string   `json:"controlIds"`
	Layers         []layerRef `json:"layers"`
}
type control struct {
	ID               string   `json:"id"`
	Event            string   `json:"event"`
	Class            string   `json:"class"`
	MinSamplePPM     int      `json:"minSamplePPM"`
	MaxSamplePPM     int      `json:"maxSamplePPM"`
	DefaultSamplePPM int      `json:"defaultSamplePPM"`
	MaxWorkMillis    int      `json:"maxWorkMillis"`
	OnFault          string   `json:"onFault"`
	AllowMaintenance bool     `json:"allowMaintenance"`
	TunableBy        []string `json:"tunableBy"`
}
type layerRef struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	TargetID      string   `json:"targetId"`
	Path          string   `json:"path"`
	KeyID         string   `json:"keyId"`
	Revision      int      `json:"revision"`
	PayloadSHA256 string   `json:"payloadSHA256"`
	MaxAgeSeconds int      `json:"maxAgeSeconds"`
	Priority      int      `json:"priority"`
	ControlIDs    []string `json:"controlIds"`
}
type envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []signature `json:"signatures"`
}
type signature struct {
	Sig   string `json:"sig"`
	KeyID string `json:"keyid,omitempty"`
}
type layer struct {
	Version     string       `json:"version"`
	ID          string       `json:"id"`
	Kind        string       `json:"kind"`
	TargetID    string       `json:"targetId"`
	Revision    int          `json:"revision"`
	IssuedAt    string       `json:"issuedAt"`
	ExpiresAt   string       `json:"expiresAt"`
	Controls    []constraint `json:"controls"`
	Preferences []preference `json:"preferences"`
}
type constraint struct {
	ID               string   `json:"id"`
	MinSamplePPM     int      `json:"minSamplePPM"`
	MaxSamplePPM     int      `json:"maxSamplePPM"`
	MaxWorkMillis    int      `json:"maxWorkMillis"`
	OnFault          string   `json:"onFault"`
	AllowMaintenance bool     `json:"allowMaintenance"`
	TunableBy        []string `json:"tunableBy"`
}
type preference struct {
	ControlID string `json:"controlId"`
	SamplePPM int    `json:"samplePPM"`
}

type effectiveControl struct {
	ID               string   `json:"id"`
	Event            string   `json:"event"`
	Class            string   `json:"class"`
	MinSamplePPM     int      `json:"minSamplePPM"`
	MaxSamplePPM     int      `json:"maxSamplePPM"`
	SamplePPM        int      `json:"samplePPM"`
	MaxWorkMillis    int      `json:"maxWorkMillis"`
	OnFault          string   `json:"onFault"`
	AllowMaintenance bool     `json:"allowMaintenance"`
	TunableBy        []string `json:"tunableBy"`
	ConstrainedBy    []string `json:"constrainedBy"`
	TunedBy          string   `json:"tunedBy,omitempty"`
}
