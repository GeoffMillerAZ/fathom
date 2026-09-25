package navigationproof

import "list"
import "strings"

#Hash: string & =~"^[a-f0-9]{64}$"
#Text: string & strings.MaxRunes(2048)
#Label: string & strings.MaxRunes(256)
#Path: string & strings.MaxRunes(512)
#CaseSpec: {
	id!: string & =~"^[a-z0-9-]{1,64}$"
	width!: int & >0 & <=4096
	theme!: "light" | "dark"
	input!: "keyboard" | "pointer" | "touch"
	zoom!: 1 | 2
}
#Color: [number, number, number]
#Control: {
	href!: #Path
	label!: #Label
	visible!: bool
	width!: number
	height!: number
	client_width!: number
	scroll_width!: number
	normal_fg!: #Color
	normal_bg!: #Color
	hover_fg!: #Color
	hover_bg!: #Color
	focus_fg!: #Color
	focus_bg!: #Color
	outline_bg!: #Color
	outline_rgb!: #Color
	outline_width!: number
	outline_style!: string & strings.MaxRunes(32)
	focus_visible!: bool
	focused_href!: #Path
}
#Step: {
	from_path!: #Path
	target_key!: string & strings.MaxRunes(64)
	input!: string & strings.MaxRunes(32)
	path!: #Path
	status!: int
	specimen_current!: string & strings.MaxRunes(64)
	current_href!: #Path
	current_count!: int
	tabs_to_target!: int
}
#Observation: {
	spec!: #CaseSpec
	inner_width!: number
	outer_width!: number
	dpr!: number
	viewport_scale!: number
	fonts_loaded!: bool
	scroll_width!: number
	controls!: [...#Control] & list.MaxItems(8)
	steps!: [...#Step] & list.MaxItems(8)
	back!: #Step
}
#Contract: {
	schema_version!: 1
	task!: "p06-links-input-contrast-v1"
	source_sha256!: "01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31"
	artifact_sha256!: #Hash
	prepare_sha256!: #Hash
	binary_sha256!: #Hash
	css_sha256!: #Hash
	runner_sha256!: #Hash
	oracle_sha256!: #Hash
	chrome_version!: "152.0.7977.84"
	playwright_version!: "1.63.0"
	platform!: "darwin"
	arch!: "arm64"
	origin!: string & =~"^http://127\\.0\\.0\\.1:[1-9][0-9]{0,4}$"
	cases!: [...#CaseSpec] & list.MaxItems(32)
}
#Report: {
	schema_version!: 1
	contract_sha256!: #Hash
	chrome_version!: "152.0.7977.84"
	playwright_version!: "1.63.0"
	cases!: [...#Observation] & list.MaxItems(32)
	errors!: [...#Text] & list.MaxItems(64)
	forbidden_requests!: [...#Text] & list.MaxItems(64)
}
