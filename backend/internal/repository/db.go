package repository

import (
	"database/sql"
)

type DB struct {
	Conn *sql.DB
}

func NewDB(conn *sql.DB) *DB {
	return &DB{Conn: conn}
}
