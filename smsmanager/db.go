package smsmanager

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nrmodule/atserial"
)

type SMSRecord struct {
	atserial.NRModuleSMS
	DBID    int64
	CreatAt time.Time
}

type SMSDatabase struct {
	db *sql.DB
}

func NewSMSDatabase(dbPath string) (*SMSDatabase, error) {

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.New("database not found")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS sms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT NOT NULL,
		content TEXT NOT NULL,
		status TEXT,
		received_at DATETIME NOT NULL,
		module_indices INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(sender, content, received_at)
	);
	CREATE INDEX IF NOT EXISTS idx_received_at ON sms(received_at);
	`

	_, err = db.Exec(schema)
	if err != nil {
		db.Close()
		log.Println(err)
		return nil, errors.New("database init failed")
	}

	return &SMSDatabase{db: db}, nil
}

func (sdb *SMSDatabase) getSMSID(sms atserial.NRModuleSMS) (int64, error) {

	var id int64

	query := `
	SELECT id FROM sms
	WHERE sender = ? AND content = ? AND received_at = ?
	LIMIT 1
	`

	err := sdb.db.QueryRow(query, sms.Sender, sms.Text, sms.Date).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (sdb *SMSDatabase) InsertSMS(sms atserial.NRModuleSMS) (dbID int64, isNew bool, err error) {
	
    tx, err := sdb.db.Begin()
    if err != nil {
        return 0, false, err
    }
    defer tx.Rollback()

    var id int64
    err = tx.QueryRow("SELECT id FROM sms WHERE sender = ? AND content = ? AND received_at = ? LIMIT 1",
        sms.Sender, sms.Text, sms.Date).Scan(&id)
    
    if err == nil {
        return id, false, nil
    }
    
    if err != sql.ErrNoRows {
        return 0, false, err
    }

    result, err := tx.Exec("INSERT INTO sms (sender, content, status, received_at, module_indices) VALUES (?, ?, ?, ?, ?)",
        sms.Sender, sms.Text, sms.Status, sms.Date, sms.Indices)
    
    if err != nil {
        return 0, false, err
    }

    id, err = result.LastInsertId()
    if err != nil {
        return 0, false, err
    }

    err = tx.Commit()
    return id, true, err
}

func (sdb *SMSDatabase) GetSMSByID(id int64) (*SMSRecord, error) {

	query := `
		SELECT id, sender, content, status, received_at, module_indices, created_at
		FROM sms WHERE id = ?
	`

	var record SMSRecord
	err := sdb.db.QueryRow(query, id).Scan(
		&record.DBID,
		&record.Sender,
		&record.Text,
		&record.Status,
		&record.Date,
		&record.Indices,
		&record.CreatAt,
	)

	if err != nil {
		return nil, errors.New("sms doesn't exist")
	}

	return &record, err
}

func (sdb *SMSDatabase) GetSMSByRange(startID int64, endID int64) ([]*SMSRecord, error) {

	if startID > endID {
		return nil, errors.New("range error")
	}

	query := `
	SELECT id, sender, content, status, received_at, module_indices, created_at
	FROM sms
	WHERE id >= ? AND id <= ?
	ORDER BY id ASC
	`
	rows, err := sdb.db.Query(query, startID, endID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*SMSRecord
	for rows.Next() {
		var record SMSRecord
		err := rows.Scan(
			&record.DBID,
			&record.Sender,
			&record.Text,
			&record.Status,
			&record.Date,
			&record.Indices,
			&record.CreatAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, &record)
	}
	return records, rows.Err()
}

func (sdb *SMSDatabase) GetSMSCount() (int64, error) {
	var count int64
	err := sdb.db.QueryRow("SELECT COUNT(*) FROM sms").Scan(&count)
	return count, err
}

func (sdb *SMSDatabase) Close() error {
	return sdb.db.Close()
}
