package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"

	_ "github.com/go-sql-driver/mysql"

	"fmt"
	"log"
	"os"

	cid "github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash"
)

type TableHashData struct {
	TableName string    `json:"table_name"`
	Rows      []RowData `json:"rows"`
}

type TableData struct {
	TableName string              `json:"table_name"`
	Cells     []map[string]string `json:"records"`
}

type RowData struct {
	RowHash string            `json:"row_hash"`
	Cells   map[string]string `json:"cells"`
}

type columnsNameType struct {
	Name string
	Type string
}

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage: ./main <db_username> <db_password> <db_name> <table_name>")
		os.Exit(1)
	}

	dbUsername := os.Args[1]
	fmt.Println("dbusername:" + dbUsername)
	dbPassword := os.Args[2]
	fmt.Println("pass:" + dbPassword)
	dbName := os.Args[3]
	fmt.Println("dbName:" + dbName)
	tableName := os.Args[4]
	fmt.Println("tableName:" + tableName)
	fmt.Println("==========================")

	// Connect to MySQL
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", dbUsername, dbPassword, dbName))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get table columns and types
	columns, err := getTableColumns(db, tableName)
	if err != nil {
		log.Fatal(err)
	}

	for _, col := range columns {
		fmt.Println(col.Name, col.Type)
	}
	fmt.Println("==========================")
	// Get table data
	rows, err := getTableData(db, tableName, columns)
	if err != nil {
		log.Fatal(err)
	}
	var cellsData []map[string]string

	for _, row := range rows {
		cellsData = append(cellsData, row.Cells)
	}

	jsonData := TableData{
		TableName: tableName,
		Cells:     cellsData,
	}
	jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonBytes))
	fmt.Println("==========================")

	jsonHashData := TableHashData{
		TableName: tableName,
		Rows:      rows,
	}

	jsonHashBytes, err := json.MarshalIndent(jsonHashData, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonHashBytes))
	fmt.Println("==========================")
}

func getTableColumns(db *sql.DB, tableName string) ([]columnsNameType, error) {
	var columns []columnsNameType

	rows, err := db.Query(fmt.Sprintf("DESCRIBE %s", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var columnInfo columnsNameType
		var columnName, columnType, columnNull, columnKey, columnExtra string
		var columnDefault sql.NullString
		if err := rows.Scan(&columnName, &columnType, &columnNull, &columnKey, &columnDefault, &columnExtra); err != nil {
			return nil, err
		}
		columnInfo.Name = columnName
		columnInfo.Type = columnType
		columns = append(columns, columnInfo)
	}

	return columns, nil
}

func getTableData(db *sql.DB, tableName string, columns []columnsNameType) ([]RowData, error) {

	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableData []RowData
	// Initialize the running hash
	runningHash := ""

	for rows.Next() {
		var cellsResult = make(map[string]string)

		var cells = make([]string, len(columns))
		var cellValues = make([]interface{}, len(columns))
		for i := range cellValues {
			cellValues[i] = &cells[i]
		}

		if err := rows.Scan(cellValues...); err != nil {
			return nil, err
		}

		for i, col := range columns {
			cellsResult[col.Name] = cells[i]
		}

		// Convert values to JSON
		jsonData, err := json.Marshal(cells)
		if err != nil {
			return nil, err
		}

		hash := calculateHash(runningHash + string(jsonData))
		runningHash = hash

		cid, err := createCID(hash)
		if err != nil {
			fmt.Println("Error creating CID:", err)
			return nil, err
		}

		rowData := RowData{
			RowHash: cid,
			Cells:   cellsResult,
		}
		tableData = append(tableData, rowData)
	}

	return tableData, nil
}

func calculateHash(data string) string {
	hasher := sha256.New()
	hasher.Write([]byte(data))
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

func createCID(data string) (string, error) {
	hash := calculateHash(data)

	// Convert the hash to multihash format
	mHash, err := multihash.Encode([]byte(hash), multihash.SHA2_256)
	if err != nil {
		return "", err
	}

	// Create a CID from the multihash
	contentID := cid.NewCidV1(cid.Raw, mHash)
	return contentID.String(), nil
}
