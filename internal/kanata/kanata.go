package kanata

type ReloadRequest struct {
	Wait      bool `json:"wait"`
	TimeoutMs int  `json:"timeout_ms"`
}

type ReloadMessage struct {
	Reload ReloadRequest `json:"Reload"`
}
