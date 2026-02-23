package db

import (
	"database/sql"

	"tangled.org/core/appview/models"
)

func GetPunchcardPreference(e Execer, did string) (models.PunchcardPreference, error) {
	preference := models.PunchcardPreference{
		Did: did,
	}

	hideMine := 0
	hideOthers := 0

	err := e.QueryRow(
		`select id, hide_mine, hide_others from punchcard_preferences where user_did = ?`,
		did,
	).Scan(&preference.ID, &hideMine, &hideOthers)
	if err == sql.ErrNoRows {
		return preference, nil
	}

	preference.HideMine = hideMine > 0
	preference.HideOthers = hideOthers > 0

	if err != nil {
		return preference, err
	}

	return preference, nil
}

func UpsertPunchcardPreference(e Execer, did string, hideMine, hideOthers bool) error {
	_, err := e.Exec(
		`insert or replace into punchcard_preferences (
			user_did,
			hide_mine,
		 	hide_others
		)
		values (?, ?, ?)`,
		did,
		hideMine,
		hideOthers,
	)
	if err != nil {
		return err
	}

	return nil
}
