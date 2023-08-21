package main

func getAndDeleteRandomNews() (string, error) {
	rows, err := db.Query("SELECT text FROM news ORDER BY RANDOM() LIMIT 1")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var news string
	for rows.Next() {
		err = rows.Scan(&news)
		if err != nil {
			return "", err
		}
	}
	_, err = db.Exec("DELETE FROM news WHERE text = $1", news)
	if err != nil {
		return "", err
	}
	return news, nil
}
