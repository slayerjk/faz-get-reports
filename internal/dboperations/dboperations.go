package dboperations

import (
	// sqllite support
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// get list of unprocessed values in db('Processed' column = NULL)
func GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn string) ([]string, error) {
	result := make([]string, 0)

	// open db file
	db, err := sql.Open("sqlite3", "file:"+dbFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open db file(%s):\n\t%v", dbFile, err)
	}
	defer db.Close()

	// get select result of unprocessed values('Processed' column = NULL)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NULL", dbValueColumn, dbTable, dbProcessedColumn)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to make select of unprocessed values:\n\t%v", err)
	}
	defer rows.Close()

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

// update processed value(0 - for failed, 1 - for succeeded)
func UpdDbValue(dbFile, dbTable, dbValueColumn, dbColumnToUpd, dbProcessedDateColumn, valueToUpd string, updTo int) error {
	// open db file
	db, err := sql.Open("sqlite3", "file:"+dbFile)
	if err != nil {
		return fmt.Errorf("failed to open db file(%s):\n\t%v", dbFile, err)
	}
	defer db.Close()

	// upd db value
	processedDate := time.Now().Format("02.01.2006 15:04:05")
	query := fmt.Sprintf(
		"UPDATE %s SET %s = %d, %s = '%s' WHERE %s = '%s'",
		dbTable, dbColumnToUpd, updTo, dbProcessedDateColumn, processedDate, dbValueColumn, valueToUpd)
	fmt.Println(query)
	result, errU := db.Exec(query)
	if errU != nil {
		return errU
	}

	// if 0 affected rows than something wrong
	affectedRows, _ := result.RowsAffected()
	if affectedRows == 0 {
		return fmt.Errorf("0 affected rows, recheck the query:\n\t%s", query)
	}

	db.Close()
	return nil
}
