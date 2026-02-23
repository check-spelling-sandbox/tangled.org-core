package models

type PunchcardPreference struct {
	ID         int
	Did        string
	HideMine   bool
	HideOthers bool
}
