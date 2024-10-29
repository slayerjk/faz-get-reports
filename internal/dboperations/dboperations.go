package dboperations

import (
	// sqllite support
	"database/sql"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// get list of unprocessed values in db
func GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn string) ([]string, error) {
	result := make([]string, 0)

	// open db file
	db, err := sql.Open("sqlite3", "file:"+dbFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open db file(%s):\n\t%v", dbFile, err)
	}
	defer db.Close()

	// get select result of unprocessed values
	// query := fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NULL", dbValueColumn, dbTable, dbProcessedColumn)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s=3", dbValueColumn, dbTable, dbProcessedColumn)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to make select of unprocessed values:\n\t%v", err)
	}

	for rows.Next() {
		var value string

		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("failed to scan rows of select of unprocessed values:\n\t%v", err)
		}
		result = append(result, value)
	}

	db.Close()
	return result, nil
}
